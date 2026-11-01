package app.goodiecat;

import android.content.Context;
import android.content.SharedPreferences;
import android.net.Uri;

import java.net.HttpURLConnection;
import java.net.URL;
import java.util.ArrayList;
import java.util.List;

// Where the user's Goodiecat lives, as entered on the setup screen: a home
// address (LAN), an optional remote one (Tailscale, VPN, ...) and the app
// token that unlocks start / stop / restart.
final class Server {
    static final String PREFS = "server";
    static final String TOKEN_HEADER = "X-Goodiecat-Token";

    final String home;
    final String remote;
    final String token;

    private Server(String home, String remote, String token) {
        this.home = home;
        this.remote = remote;
        this.token = token;
    }

    static Server load(Context ctx) {
        SharedPreferences p = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE);
        return new Server(p.getString("home", ""), p.getString("remote", ""), p.getString("token", ""));
    }

    static void save(Context ctx, String home, String remote, String token) {
        ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
                .putString("home", normalize(home))
                .putString("remote", normalize(remote))
                .putString("token", token.trim())
                .apply();
    }

    boolean isConfigured() {
        return !home.isEmpty();
    }

    // "192.168.1.10:8080/" -> "http://192.168.1.10:8080"
    static String normalize(String address) {
        String a = address.trim();
        if (a.isEmpty()) return "";
        if (!a.contains("://")) a = "http://" + a;
        while (a.endsWith("/")) a = a.substring(0, a.length() - 1);
        return a;
    }

    List<String> bases() {
        List<String> list = new ArrayList<>();
        if (!home.isEmpty()) list.add(home);
        if (!remote.isEmpty()) list.add(remote);
        return list;
    }

    // the first address that answers, or null; call off the main thread
    String reachableBase() {
        for (String base : bases()) {
            if (reachable(base, base.equals(home) ? 1500 : 5000)) return base;
        }
        return null;
    }

    static boolean reachable(String base, int timeoutMs) {
        try {
            HttpURLConnection c = (HttpURLConnection) new URL(base + "/api/healthz").openConnection();
            c.setConnectTimeout(timeoutMs);
            c.setReadTimeout(5000);
            int code = c.getResponseCode();
            c.disconnect();
            return code < 500;
        } catch (Exception e) {
            return false;
        }
    }

    static String host(String base) {
        return base.isEmpty() ? "" : Uri.parse(base).getHost();
    }

    // a link to one of the user's machines, under either address
    boolean ownsHost(String host) {
        return host != null && (host.equalsIgnoreCase(host(home)) || (!remote.isEmpty() && host.equalsIgnoreCase(host(remote))));
    }
}
