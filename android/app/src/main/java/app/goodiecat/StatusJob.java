package app.goodiecat;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.job.JobInfo;
import android.app.job.JobParameters;
import android.app.job.JobScheduler;
import android.app.job.JobService;
import android.content.ComponentName;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

// Checks the server's containers every ~15 minutes (Android's minimum for
// background jobs) and notifies when one that was running goes down, and
// again when it comes back. Containers that were already stopped are ignored.
public class StatusJob extends JobService {
    static final int JOB_ID = 1;
    static final String CHANNEL = "service-alerts";
    static final String PREFS = "status";
    static final String KEY_ALERT_DOWN = "alert_down";
    static final String KEY_ALERT_UP = "alert_up";

    static boolean alertDown(SharedPreferences prefs) {
        return prefs.getBoolean(KEY_ALERT_DOWN, true);
    }

    static boolean alertUp(SharedPreferences prefs) {
        return prefs.getBoolean(KEY_ALERT_UP, false);
    }

    static void cancel(Context ctx) {
        ctx.getSystemService(JobScheduler.class).cancel(JOB_ID);
    }

    static void schedule(Context ctx) {
        JobScheduler js = ctx.getSystemService(JobScheduler.class);
        if (js.getPendingJob(JOB_ID) != null) return;
        js.schedule(new JobInfo.Builder(JOB_ID, new ComponentName(ctx, StatusJob.class))
                .setRequiredNetworkType(JobInfo.NETWORK_TYPE_ANY)
                .setPeriodic(15 * 60 * 1000L)
                .setPersisted(true)
                .build());
    }

    @Override
    public boolean onStartJob(JobParameters params) {
        new Thread(() -> {
            try {
                check(this);
            } catch (Exception ignored) {
                // server unreachable (phone offline, Tailscale off): stay quiet
            }
            jobFinished(params, false);
        }).start();
        return true;
    }

    @Override
    public boolean onStopJob(JobParameters params) {
        return true;
    }

    static void check(Context ctx) throws Exception {
        JSONArray list = fetch(ctx).getJSONArray("data");
        // name -> problem ("" when healthy)
        Map<String, String> now = new HashMap<>();
        for (int i = 0; i < list.length(); i++) {
            JSONObject c = list.getJSONObject(i);
            String problem = "";
            if (!"running".equals(c.optString("state"))) problem = c.optString("status", "stopped");
            else if ("unhealthy".equals(c.optString("health"))) problem = "unhealthy";
            now.put(c.getString("name"), problem);
        }

        SharedPreferences prefs = ctx.getSharedPreferences(PREFS, MODE_PRIVATE);
        boolean alertDown = alertDown(prefs);
        boolean alertUp = alertUp(prefs);
        if (!alertDown && !alertUp) return;
        boolean firstRun = !prefs.contains("up");
        Set<String> wasUp = prefs.getStringSet("up", new HashSet<>());
        Set<String> wasDown = prefs.getStringSet("down", new HashSet<>());
        Set<String> up = new HashSet<>();
        Set<String> down = new HashSet<>();

        for (Map.Entry<String, String> e : now.entrySet()) {
            if (e.getValue().isEmpty()) {
                up.add(e.getKey());
                if (wasDown.contains(e.getKey())) {
                    if (alertUp) notify(ctx, e.getKey(), e.getKey() + " is back up", "Running again");
                    else clear(ctx, e.getKey()); // its "down" alert is no longer true
                }
            } else if (wasUp.contains(e.getKey()) || wasDown.contains(e.getKey())) {
                down.add(e.getKey());
                if (alertDown && !firstRun && wasUp.contains(e.getKey())) notify(ctx, e.getKey(), e.getKey() + " is down", e.getValue());
            }
        }
        // a running container that vanished entirely (removed, or mid-recreate)
        for (String name : wasUp) {
            if (!now.containsKey(name)) {
                down.add(name);
                if (alertDown) notify(ctx, name, name + " is gone", "The container no longer exists");
            }
        }
        for (String name : wasDown) {
            if (!now.containsKey(name)) down.add(name);
        }

        prefs.edit().putStringSet("up", up).putStringSet("down", down).apply();
    }

    static JSONObject fetch(Context ctx) throws Exception {
        Server server = Server.load(ctx);
        Exception last = null;
        for (String base : server.bases()) {
            try {
                HttpURLConnection c = (HttpURLConnection) new URL(base + "/api/goodiecat/containers").openConnection();
                c.setConnectTimeout(base.equals(server.home) ? 2000 : 5000);
                if (!server.token.isEmpty()) c.setRequestProperty(Server.TOKEN_HEADER, server.token);
                c.setReadTimeout(8000);
                try (InputStream in = c.getInputStream()) {
                    ByteArrayOutputStream out = new ByteArrayOutputStream();
                    byte[] buf = new byte[8192];
                    for (int n; (n = in.read(buf)) > 0; ) out.write(buf, 0, n);
                    JSONObject json = new JSONObject(out.toString("UTF-8"));
                    if (json.optBoolean("ok")) return json;
                } finally {
                    c.disconnect();
                }
            } catch (Exception e) {
                last = e;
            }
        }
        throw last != null ? last : new Exception("Goodiecat unreachable");
    }

    static void clear(Context ctx, String name) {
        ctx.getSystemService(NotificationManager.class).cancel(name.hashCode());
    }

    static void notify(Context ctx, String name, String title, String text) {
        NotificationManager nm = ctx.getSystemService(NotificationManager.class);
        nm.createNotificationChannel(new NotificationChannel(
                CHANNEL, "Service alerts", NotificationManager.IMPORTANCE_HIGH));
        PendingIntent open = PendingIntent.getActivity(ctx, 0,
                new Intent(ctx, MainActivity.class), PendingIntent.FLAG_IMMUTABLE);
        // one notification per container: "back up" replaces its "down" alert
        nm.notify(name.hashCode(), new Notification.Builder(ctx, CHANNEL)
                .setSmallIcon(R.drawable.ic_notify)
                .setContentTitle(title)
                .setContentText(text)
                .setContentIntent(open)
                .setAutoCancel(true)
                .build());
    }
}
