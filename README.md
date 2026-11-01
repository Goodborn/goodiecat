<p align="center"><img width="120" src="internal/dynacat/static/favicon.svg" alt="Goodiecat"></p>
<h1 align="center">Goodiecat</h1>
<p align="center">Your home server at a glance, on your PC and your phone.</p>
<p align="center">
  <a href="https://github.com/Goodborn/goodiecat/releases/latest"><img src="https://img.shields.io/github/v/release/Goodborn/goodiecat?label=release&color=8b5cf6" alt="Latest release"></a>
  <a href="https://github.com/Goodborn/goodiecat/releases/latest/download/Goodiecat.apk"><img src="https://img.shields.io/badge/Android-app-ec4899?logo=android&logoColor=white" alt="Android app"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-64748b" alt="AGPL-3.0"></a>
</p>

<p align="center"><img src=".github/screenshots/hero.png" alt="Goodiecat on a desktop browser and a phone"></p>

<table align="center"><tr>
<td align="center">🖥️<br><b>Server card</b><br><sub>CPU, memory, disks, temps</sub></td>
<td align="center">🟢<br><b>Live services</b><br><sub>green running, red down</sub></td>
<td align="center">📱<br><b>Phone ready</b><br><sub>everything stacks</sub></td>
<td align="center">🔔<br><b>Down alerts</b><br><sub>from the app</sub></td>
</tr></table>

## 📱 The app

<p align="center"><img src=".github/screenshots/app-screens.png" alt="Tap a service to open, start, restart or stop it; alert settings; setup screen"></p>

<p align="center"><img width="72" src=".github/screenshots/app-icon.png" alt="App icon"><br>
<a href="https://github.com/Goodborn/goodiecat/releases/latest/download/Goodiecat.apk"><b>Download Goodiecat.apk</b></a> · Android 8+</p>

## 🚀 Run it

```bash
git clone https://github.com/Goodborn/goodiecat && cd goodiecat/examples
mkdir -p config && cp goodiecat.yml config/
echo "GOODIECAT_APP_TOKEN=$(openssl rand -hex 24)" > .env
docker compose up -d        # → http://your-server:8080
```

Config, API and app details: **[docs/goodiecat.md](docs/goodiecat.md)** · Widgets: **[Dynacat docs](https://dynacat.artur.zone/configuration)**

<sub>Fork of <a href="https://github.com/Panonim/dynacat">Dynacat</a>, built on <a href="https://github.com/glanceapp/glance">Glance</a>. All credit for the base goes to them. AGPL-3.0. Screenshots use demo data.</sub>
