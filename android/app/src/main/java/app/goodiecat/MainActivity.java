package app.goodiecat;

import android.Manifest;
import android.app.Activity;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.graphics.Insets;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.view.WindowInsets;
import android.webkit.JavascriptInterface;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.Toast;
import android.window.OnBackInvokedDispatcher;

import androidx.swiperefreshlayout.widget.SwipeRefreshLayout;

import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;

public class MainActivity extends Activity {
    // matches Goodiecat's default background so the system bar areas blend in
    static final int BACKGROUND = 0xFF0F1014;
    static final String SETUP_PAGE = "file:///android_asset/setup.html";

    WebView web;
    SwipeRefreshLayout swipe;
    boolean failed;
    Server server;
    // the address that currently answers: the home one, or the remote one when away
    String base = "";
    // assets/dashboard.js: settings gear + service panel, injected per page
    String dashboardJs;
    // one of dashboard.js's panels is showing, so Back should close it
    volatile boolean sheetOpen;
    // the bridge only acts for the dashboard and the setup page, never for a
    // service's own page opened inside the app
    volatile boolean trustedPage;
    // a service page was opened with "Open" (in the app), so its own links
    // stay in the app too until we're back on the dashboard
    boolean serviceInApp;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        server = Server.load(this);
        web = new WebView(this);
        web.getSettings().setJavaScriptEnabled(true);
        web.getSettings().setDomStorageEnabled(true);
        web.addJavascriptInterface(new Bridge(), "GoodiecatApp");
        dashboardJs = readAsset("dashboard.js");
        web.setWebViewClient(new WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest req) {
                Uri url = req.getUrl();
                if (isDashboard(url.toString())) return false;
                if (server.ownsHost(url.getHost())) {
                    Uri reachable = viaWorkingAddress(url);
                    if (serviceInApp) {
                        if (reachable.equals(url)) return false;
                        view.loadUrl(reachable.toString());
                    } else {
                        openInBrowser(reachable);
                    }
                    return true;
                }
                // everything else (GitHub, IMDb, ...) opens in the browser
                openInBrowser(url);
                return true;
            }

            @Override
            public void doUpdateVisitedHistory(WebView view, String url, boolean isReload) {
                trustedPage = isDashboard(url) || url.startsWith(SETUP_PAGE);
                if (isDashboard(url)) serviceInApp = false;
                // pull-to-refresh fights the inner scrolling of the services' own
                // pages, so keep it to the dashboard, setup and error pages
                swipe.setEnabled(failed || trustedPage);
            }

            @Override
            public void onPageFinished(WebView view, String url) {
                swipe.setRefreshing(false);
                if (isDashboard(url)) view.evaluateJavascript(dashboardJs, null);
            }

            @Override
            public void onReceivedError(WebView view, WebResourceRequest req, WebResourceError err) {
                if (!req.isForMainFrame()) return;
                showError();
            }
        });

        swipe = new SwipeRefreshLayout(this);
        swipe.addView(web);
        // only pull to refresh when the page itself is scrolled to the top
        swipe.setOnChildScrollUpCallback((parent, child) -> web.getScrollY() > 0);
        swipe.setOnRefreshListener(() -> {
            if (failed) connect();
            else web.reload();
        });
        swipe.setBackgroundColor(BACKGROUND);
        web.setBackgroundColor(BACKGROUND);
        // Android 15+ draws apps edge-to-edge; keep the page clear of the status
        // bar, nav bar, notch and keyboard instead of sliding under them
        if (Build.VERSION.SDK_INT >= 30) {
            swipe.setOnApplyWindowInsetsListener((v, insets) -> {
                Insets bars = insets.getInsets(WindowInsets.Type.systemBars()
                        | WindowInsets.Type.displayCutout() | WindowInsets.Type.ime());
                v.setPadding(bars.left, bars.top, bars.right, bars.bottom);
                return WindowInsets.CONSUMED;
            });
        }
        setContentView(swipe);
        // Android 16 stops calling onBackPressed for apps targeting it
        if (Build.VERSION.SDK_INT >= 33) {
            getOnBackInvokedDispatcher().registerOnBackInvokedCallback(
                    OnBackInvokedDispatcher.PRIORITY_DEFAULT, this::handleBack);
        }
        getWindow().getDecorView().setBackgroundColor(BACKGROUND);

        if (!server.isConfigured()) {
            showSetup();
            return;
        }
        // always reconnect: the working address may have changed since last time
        connect();

        // background check that alerts when a container goes down; a failure
        // here must never stop the dashboard itself from opening
        applyAlertSettings();
        // also check right away, so opening the app refreshes the alerts
        new Thread(() -> {
            try {
                StatusJob.check(getApplicationContext());
            } catch (Exception ignored) {
            }
        }).start();
    }

    boolean isDashboard(String url) {
        return url != null && !base.isEmpty() && (url.equals(base) || url.startsWith(base + "/"));
    }

    // called from assets/dashboard.js and assets/setup.html, which draw their
    // own UI in the page
    class Bridge {
        @JavascriptInterface
        public String getAlertSettings() {
            if (!trustedPage) return "{}";
            SharedPreferences prefs = getSharedPreferences(StatusJob.PREFS, MODE_PRIVATE);
            return "{\"down\":" + StatusJob.alertDown(prefs) + ",\"up\":" + StatusJob.alertUp(prefs) + "}";
        }

        @JavascriptInterface
        public void setAlertSettings(boolean down, boolean up) {
            if (!trustedPage) return;
            getSharedPreferences(StatusJob.PREFS, MODE_PRIVATE).edit()
                    .putBoolean(StatusJob.KEY_ALERT_DOWN, down)
                    .putBoolean(StatusJob.KEY_ALERT_UP, up)
                    .apply();
            runOnUiThread(MainActivity.this::applyAlertSettings);
        }

        // without a token the panel still opens pages, it just can't control
        @JavascriptInterface
        public boolean canControl() {
            return trustedPage && !server.token.isEmpty();
        }

        @JavascriptInterface
        public void control(String name, String image, String action) {
            if (!trustedPage) return;
            runOnUiThread(() -> controlContainer(name, image, action));
        }

        // the service panel's "Open" (in the app) and "Open in browser"
        @JavascriptInterface
        public void openService(String url, boolean inApp) {
            if (!trustedPage) return;
            runOnUiThread(() -> {
                Uri target = viaWorkingAddress(Uri.parse(url));
                if (inApp) {
                    serviceInApp = true;
                    web.loadUrl(target.toString());
                } else {
                    openInBrowser(target);
                }
            });
        }

        @JavascriptInterface
        public void sheetOpen(boolean open) {
            sheetOpen = open;
        }

        @JavascriptInterface
        public String serverLabel() {
            return trustedPage ? base : "";
        }

        @JavascriptInterface
        public void openServerSettings() {
            if (!trustedPage) return;
            runOnUiThread(MainActivity.this::showSetup);
        }

        @JavascriptInterface
        public String getServer() {
            if (!trustedPage) return "{}";
            try {
                return new JSONObject()
                        .put("home", server.home)
                        .put("remote", server.remote)
                        .put("hasToken", !server.token.isEmpty())
                        .toString();
            } catch (Exception e) {
                return "{}";
            }
        }

        // from the setup page: check the address answers, then save and open it
        @JavascriptInterface
        public void saveServer(String home, String remote, String token, boolean keepToken) {
            if (!trustedPage) return;
            String h = Server.normalize(home);
            String r = Server.normalize(remote);
            String t = keepToken ? server.token : token;
            new Thread(() -> {
                boolean homeOk = !h.isEmpty() && Server.reachable(h, 3000);
                boolean remoteOk = !r.isEmpty() && Server.reachable(r, 6000);
                runOnUiThread(() -> {
                    if (h.isEmpty()) {
                        setupResult(false, "Enter your Goodiecat address");
                    } else if (!homeOk && !remoteOk) {
                        setupResult(false, "Couldn't reach Goodiecat at that address");
                    } else {
                        Server.save(MainActivity.this, h, r, t);
                        server = Server.load(MainActivity.this);
                        applyAlertSettings();
                        connect();
                    }
                });
            }).start();
        }
    }

    // Dashboard links usually use the home address; swap in whichever one works
    // right now (same machine and ports, e.g. over Tailscale)
    Uri viaWorkingAddress(Uri url) {
        String want = Server.host(base);
        if (!server.ownsHost(url.getHost()) || want.isEmpty() || want.equalsIgnoreCase(url.getHost())) return url;
        String port = url.getPort() > 0 ? ":" + url.getPort() : "";
        return url.buildUpon().encodedAuthority(want + port).build();
    }

    void openInBrowser(Uri url) {
        try {
            startActivity(new Intent(Intent.ACTION_VIEW, url).addCategory(Intent.CATEGORY_BROWSABLE));
        } catch (Exception e) {
            Toast.makeText(this, "No browser to open " + url.getHost(), Toast.LENGTH_LONG).show();
        }
    }

    void setupResult(boolean ok, String message) {
        web.evaluateJavascript("window.setupResult && setupResult(" + ok + "," + JSONObject.quote(message) + ")", null);
    }

    void showSetup() {
        failed = false;
        web.loadUrl(SETUP_PAGE);
    }

    String readAsset(String file) {
        try (InputStream in = getAssets().open(file)) {
            ByteArrayOutputStream out = new ByteArrayOutputStream();
            byte[] buf = new byte[8192];
            for (int n; (n = in.read(buf)) > 0; ) out.write(buf, 0, n);
            return out.toString("UTF-8");
        } catch (IOException e) {
            return "";
        }
    }

    // the page's own toast when it's the dashboard, Android's otherwise
    void toast(String message, boolean ok) {
        if (isDashboard(web.getUrl())) {
            web.evaluateJavascript("window.gcToast && gcToast(" + JSONObject.quote(message) + "," + ok + ")", null);
        } else {
            Toast.makeText(this, message, Toast.LENGTH_LONG).show();
        }
    }

    // start / stop / restart a container through Goodiecat (service panel)
    void controlContainer(String name, String image, String action) {
        String verb = action.equals("stop") ? "Stopping" : action.equals("start") ? "Starting" : "Restarting";
        toast(verb + " " + name + "…", true);
        String target = base;
        String token = server.token;
        new Thread(() -> {
            String result;
            boolean ok = false;
            try {
                HttpURLConnection c = (HttpURLConnection) new URL(
                        target + "/api/goodiecat/containers/" + action).openConnection();
                c.setConnectTimeout(5000);
                c.setReadTimeout(40000);
                c.setRequestMethod("POST");
                c.setDoOutput(true);
                c.setRequestProperty("Content-Type", "application/json");
                c.setRequestProperty(Server.TOKEN_HEADER, token);
                JSONObject body = new JSONObject().put("name", name).put("image", image);
                try (OutputStream out = c.getOutputStream()) {
                    out.write(body.toString().getBytes("UTF-8"));
                }
                int code = c.getResponseCode();
                InputStream in = code < 400 ? c.getInputStream() : c.getErrorStream();
                ByteArrayOutputStream buf = new ByteArrayOutputStream();
                if (in != null) {
                    byte[] b = new byte[4096];
                    for (int n; (n = in.read(b)) > 0; ) buf.write(b, 0, n);
                }
                c.disconnect();
                JSONObject res = new JSONObject(buf.toString("UTF-8"));
                ok = res.optBoolean("ok");
                result = ok
                        ? name + (action.equals("stop") ? " stopped" : action.equals("start") ? " started" : " restarted")
                        : "Couldn't " + action + " " + name + ": " + res.optString("error");
            } catch (Exception e) {
                result = "Couldn't reach Goodiecat to " + action + " " + name;
            }
            String message = result;
            boolean success = ok;
            runOnUiThread(() -> {
                toast(message, success);
                // show the new state now rather than at the next widget refresh
                // (after a moment, so the toast is seen first)
                if (success) web.postDelayed(() -> {
                    if (isDashboard(web.getUrl())) web.reload();
                }, 1800);
            });
        }).start();
    }

    // background checks only run while at least one alert is switched on
    void applyAlertSettings() {
        SharedPreferences prefs = getSharedPreferences(StatusJob.PREFS, MODE_PRIVATE);
        boolean any = server.isConfigured() && (StatusJob.alertDown(prefs) || StatusJob.alertUp(prefs));
        // a failure here must never stop the dashboard itself from opening
        try {
            if (any) StatusJob.schedule(this);
            else StatusJob.cancel(this);
        } catch (RuntimeException ignored) {
        }
        if (any && Build.VERSION.SDK_INT >= 33
                && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[] {Manifest.permission.POST_NOTIFICATIONS}, 1);
        }
    }

    // picks whichever address answers, off the main thread
    void connect() {
        swipe.setRefreshing(true);
        new Thread(() -> {
            String found = server.reachableBase();
            runOnUiThread(() -> {
                if (found == null) {
                    showError();
                    return;
                }
                failed = false;
                base = found;
                web.clearHistory();
                web.loadUrl(base + "/");
            });
        }).start();
    }

    void showError() {
        failed = true;
        swipe.setRefreshing(false);
        String remoteHint = server.remote.isEmpty()
                ? "Away from home? Add a remote address (for example over Tailscale) in the server settings."
                : "Away from home? Make sure your VPN or Tailscale is on.";
        web.loadDataWithBaseURL(SETUP_PAGE, "<body style='background:#0f1014;color:#d6d9e0;font:17px sans-serif;padding:28px;line-height:1.5'>"
                + "<h2 style='margin:0 0 12px'>Can't reach Goodiecat</h2>"
                + "<p>Tried " + escape(String.join(" and ", server.bases())) + ".</p>"
                + "<p>" + remoteHint + "</p><p>Pull down to try again.</p>"
                + "<p><a style='color:#a78bfa' href='#' onclick='GoodiecatApp.openServerSettings();return false'>Server settings</a></p></body>",
                "text/html", "utf-8", null);
    }

    static String escape(String s) {
        return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;");
    }

    @Override
    public void onBackPressed() {
        handleBack();
    }

    // Back closes an open panel, then steps back through pages, then exits
    void handleBack() {
        if (sheetOpen) {
            sheetOpen = false;
            web.evaluateJavascript("window.gcCloseSheet && gcCloseSheet()", null);
        } else if (!failed && web.canGoBack()) {
            web.goBack();
        } else if (server.isConfigured() && web.getUrl() != null && web.getUrl().startsWith(SETUP_PAGE) && !base.isEmpty()) {
            web.loadUrl(base + "/");
        } else {
            finish();
        }
    }
}
