// Injected into the Goodiecat dashboard by the app (MainActivity.onPageFinished).
// Adds things that only make sense inside the app, drawn in the dashboard's
// own style: the alert settings gear beside the search icon, and a panel on
// tapping a service tile (open its page, start / stop / restart). Safe to run
// more than once.
(function () {
    if (!document.getElementById('gc-app-style')) {
        var style = document.createElement('style');
        style.id = 'gc-app-style';
        style.textContent = [
            '#gc-app-settings{position:absolute;z-index:12;top:var(--gc-search-top,12px);',
            'right:calc(max(0px,(100% - 1920px)/2) + var(--gc-search-right,16px) + 44px);',
            'width:36px;height:36px;display:flex;align-items:center;justify-content:center;padding:0;',
            'border-radius:999px;cursor:pointer;border:1px solid var(--gc-line);',
            'background:var(--gc-surface);color:var(--color-text-subdue)}',
            '#gc-app-settings svg{width:18px;height:18px}',
            '.docker-container{cursor:pointer;-webkit-tap-highlight-color:transparent}',

            // sheet: bottom panel on phones, centred card on wide screens
            '.gc-sheet-back{position:fixed;inset:0;z-index:100;display:flex;align-items:flex-end;justify-content:center;',
            'padding:12px;background:rgb(0 0 0 / .55);backdrop-filter:blur(3px);opacity:0;transition:opacity .18s}',
            '.gc-sheet-back.gc-open{opacity:1}',
            '@media (min-width:641px){.gc-sheet-back{align-items:center}}',
            '.gc-sheet{width:100%;max-width:400px;padding:18px 16px 14px;border:1px solid var(--gc-line);',
            'border-radius:16px;background:var(--color-widget-background);box-shadow:0 20px 50px -20px rgb(0 0 0 / .9);',
            'transform:translateY(16px);transition:transform .2s ease;font-family:inherit}',
            '.gc-open .gc-sheet{transform:none}',
            '.gc-sheet-head{display:flex;align-items:center;gap:12px;margin-bottom:14px}',
            '.gc-sheet-head img{width:3.4rem;height:3.4rem;flex-shrink:0}',
            '.gc-sheet-title{font-size:1.7rem;font-weight:650;color:var(--color-text-highlight);line-height:1.25}',
            '.gc-sheet-sub{display:flex;align-items:center;gap:7px;margin-top:2px;font-size:1.2rem;color:var(--color-text-subdue)}',
            '.gc-sheet-actions{display:flex;flex-direction:column;gap:8px}',
            '.gc-act{display:flex;align-items:center;gap:10px;width:100%;padding:12px 14px;border-radius:10px;',
            'border:1px solid var(--gc-line);background:var(--gc-surface);color:var(--color-text-highlight);',
            'font:inherit;font-size:1.4rem;font-weight:550;text-align:left;cursor:pointer;transition:background .15s,border-color .15s}',
            '.gc-act svg{width:18px;height:18px;flex-shrink:0;color:var(--gc-act-color)}',
            '.gc-act:active{background:color-mix(in srgb,var(--gc-act-color) 14%,var(--gc-surface))}',
            '.gc-act:disabled{opacity:.35;pointer-events:none}',
            '.gc-act-open{--gc-act-color:var(--color-primary);border-color:color-mix(in srgb,var(--color-primary) 45%,var(--gc-line));',
            'background:color-mix(in srgb,var(--color-primary) 14%,var(--gc-surface))}',
            '.gc-act-sep{height:1px;margin:4px 0;background:var(--gc-line)}',
            '.gc-act-start{--gc-act-color:var(--color-positive)}',
            '.gc-act-restart{--gc-act-color:var(--color-primary)}',
            '.gc-act-stop{--gc-act-color:var(--color-negative)}',
            '.gc-act-confirm{border-color:var(--color-negative);background:color-mix(in srgb,var(--color-negative) 18%,var(--gc-surface));color:var(--color-negative)}',
            '.gc-act-busy{opacity:.6;pointer-events:none}',
            '.gc-sheet-note{margin-top:10px;font-size:1.2rem;color:var(--color-text-subdue);text-align:center}',
            '.gc-set-btn{flex-shrink:0;padding:7px 12px;border:1px solid var(--gc-line);border-radius:999px;',
            'background:var(--gc-surface);color:var(--color-text-highlight);font:inherit;font-size:1.2rem;cursor:pointer}',
            '.gc-sheet-close{display:block;width:100%;margin-top:10px;padding:10px;border:0;background:none;',
            'font:inherit;font-size:1.3rem;color:var(--color-text-subdue);cursor:pointer}',

            // settings rows with a switch
            '.gc-set{display:flex;align-items:center;gap:14px;padding:12px 2px}',
            '.gc-set + .gc-set{border-top:1px solid var(--gc-line)}',
            '.gc-set-text{flex:1;min-width:0}',
            '.gc-set-label{font-size:1.4rem;font-weight:550;color:var(--color-text-highlight)}',
            '.gc-set-hint{font-size:1.15rem;color:var(--color-text-subdue)}',
            '.gc-switch{position:relative;flex-shrink:0;width:44px;height:26px;border-radius:999px;',
            'background:var(--color-progress-border);transition:background .2s;cursor:pointer;border:0;padding:0}',
            '.gc-switch::after{content:"";position:absolute;top:3px;left:3px;width:20px;height:20px;border-radius:50%;',
            'background:var(--color-text-highlight);transition:transform .2s}',
            '.gc-switch[aria-checked="true"]{background:var(--color-primary)}',
            '.gc-switch[aria-checked="true"]::after{transform:translateX(18px)}',

            // toast
            '.gc-toast{position:fixed;left:50%;bottom:22px;z-index:101;max-width:calc(100% - 32px);padding:10px 16px;',
            'border:1px solid var(--gc-line);border-radius:999px;background:var(--color-widget-background);',
            'box-shadow:0 10px 30px -10px rgb(0 0 0 / .8);font-size:1.3rem;color:var(--color-text-highlight);',
            'display:flex;align-items:center;gap:8px;transform:translate(-50%,20px);opacity:0;transition:all .2s}',
            '.gc-toast.gc-open{transform:translate(-50%,0);opacity:1}'
        ].join('');
        document.head.appendChild(style);
    }

    var GEAR = '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">' +
        '<path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z"/>' +
        '<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/></svg>';

    var ICONS = {
        open: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 4h6v6"/><path d="M20 4 10 14"/><path d="M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/></svg>',
        start: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M8 5.14v13.72a1 1 0 0 0 1.5.86l11-6.86a1 1 0 0 0 0-1.72l-11-6.86A1 1 0 0 0 8 5.14Z"/></svg>',
        restart: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 0 1 15.3-6.4L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-15.3 6.4L3 16"/><path d="M3 21v-5h5"/></svg>',
        stop: '<svg viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>'
    };

    function esc(s) {
        return String(s).replace(/[&<>"]/g, function (ch) {
            return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[ch];
        });
    }

    // ---- sheet + toast ----

    var sheet = null;

    function openSheet(html) {
        closeSheet(true);
        sheet = document.createElement('div');
        sheet.className = 'gc-sheet-back';
        sheet.innerHTML = '<div class="gc-sheet" role="dialog">' + html +
            '<button class="gc-sheet-close" data-close>Close</button></div>';
        sheet.addEventListener('click', function (e) {
            if (e.target === sheet || e.target.hasAttribute('data-close')) closeSheet();
        });
        document.body.appendChild(sheet);
        requestAnimationFrame(function () { sheet.classList.add('gc-open'); });
        GoodiecatApp.sheetOpen(true);
        return sheet;
    }

    function closeSheet(now) {
        if (!sheet) return;
        var el = sheet;
        sheet = null;
        GoodiecatApp.sheetOpen(false);
        if (now) { el.remove(); return; }
        el.classList.remove('gc-open');
        setTimeout(function () { el.remove(); }, 200);
    }
    window.gcCloseSheet = closeSheet;

    var toastTimer = null;
    window.gcToast = function (message, ok) {
        var t = document.querySelector('.gc-toast');
        if (!t) {
            t = document.createElement('div');
            t.className = 'gc-toast';
            document.body.appendChild(t);
        }
        t.innerHTML = '<span class="gc-dot ' + (ok === false ? 'gc-dot-bad' : 'gc-dot-ok') + '"></span>' + esc(message);
        requestAnimationFrame(function () { t.classList.add('gc-open'); });
        clearTimeout(toastTimer);
        toastTimer = setTimeout(function () { t.classList.remove('gc-open'); }, 3200);
    };

    // ---- settings ----

    function openSettings() {
        var s = JSON.parse(GoodiecatApp.getAlertSettings());
        var el = openSheet(
            '<div class="gc-sheet-head"><div><div class="gc-sheet-title">Settings</div>' +
            '<div class="gc-sheet-sub">Alerts are checked about every 15 minutes in the background</div></div></div>' +
            row('down', 'Service goes down', 'A running container stops or turns unhealthy', s.down) +
            row('up', 'Service is back up', 'It recovers after going down', s.up) +
            '<div class="gc-set"><div class="gc-set-text"><div class="gc-set-label">Server</div>' +
            '<div class="gc-set-hint">' + esc(GoodiecatApp.serverLabel()) + '</div></div>' +
            '<button class="gc-set-btn" data-server>Change</button></div>');
        el.querySelector('[data-server]').addEventListener('click', function () {
            closeSheet(true);
            GoodiecatApp.openServerSettings();
        });
        el.querySelectorAll('.gc-switch').forEach(function (sw) {
            sw.addEventListener('click', function () {
                sw.setAttribute('aria-checked', sw.getAttribute('aria-checked') === 'true' ? 'false' : 'true');
                var on = function (k) { return el.querySelector('[data-key="' + k + '"]').getAttribute('aria-checked') === 'true'; };
                GoodiecatApp.setAlertSettings(on('down'), on('up'));
            });
        });
    }

    function row(key, label, hint, on) {
        return '<div class="gc-set"><div class="gc-set-text"><div class="gc-set-label">' + label + '</div>' +
            '<div class="gc-set-hint">' + hint + '</div></div>' +
            '<button class="gc-switch" role="switch" data-key="' + key + '" aria-checked="' + !!on + '" aria-label="' + label + '"></button></div>';
    }

    // the dashboard can redraw its body, so keep re-adding the gear if it goes
    function addGear() {
        var body = document.querySelector('.body-content');
        if (!body || document.getElementById('gc-app-settings')) return;
        var btn = document.createElement('button');
        btn.id = 'gc-app-settings';
        btn.setAttribute('aria-label', 'App settings');
        btn.innerHTML = GEAR;
        btn.onclick = openSettings;
        body.appendChild(btn);
    }
    addGear();
    if (window.__gcSetup) return;
    window.__gcSetup = true;
    setInterval(addGear, 3000);

    // ---- tapping a service tile ----

    function tileInfo(tile) {
        var title = tile.querySelector('.size-title-dynamic');
        var popover = tile.querySelector('[data-popover-html]');
        var image = popover && popover.querySelector('.color-highlight');
        var state = tile.querySelector('[data-popover-text]');
        var icon = tile.querySelector('.docker-container-icon');
        var link = tile.querySelector('a[href]');
        return {
            url: link ? link.href : '',
            name: title ? title.textContent.trim() : '',
            image: image ? image.textContent.trim() : '',
            state: state ? state.getAttribute('data-popover-text') : '',
            status: popover && popover.children[1] ? popover.children[1].textContent.trim() : '',
            icon: icon ? icon.outerHTML : ''
        };
    }

    function openContainerMenu(info) {
        var running = info.state === 'running';
        var canControl = GoodiecatApp.canControl();
        var el = openSheet(
            '<div class="gc-sheet-head">' + info.icon + '<div><div class="gc-sheet-title">' + esc(info.name) + '</div>' +
            '<div class="gc-sheet-sub"><span class="gc-dot ' + (running ? 'gc-dot-ok' : 'gc-dot-bad') + '"></span>' +
            esc(info.status || info.state) + '</div></div></div>' +
            '<div class="gc-sheet-actions">' +
            (info.url
                ? '<button class="gc-act gc-act-open" data-action="open">' + ICONS.open + '<span>Open ' + esc(info.name) +
                  '</span></button><div class="gc-act-sep"></div>'
                : '') +
            button('start', 'Start', running || !canControl) +
            button('restart', 'Restart', !running || !canControl) +
            button('stop', 'Stop', !running || !canControl) +
            '</div>' +
            (canControl ? '' : '<div class="gc-sheet-note">Add your app token in Server settings (gear icon) to start, stop and restart.</div>'));
        // the server knows which ones may only be restarted (the dashboard itself,
        // anything in goodiecat.docker.restart-only)
        if (canControl && running) {
            fetch('/api/goodiecat/containers', { credentials: 'same-origin' })
                .then(function (r) { return r.json(); })
                .then(function (res) {
                    var list = (res && res.data) || [];
                    var hit = list.filter(function (c) { return c.name.toLowerCase() === info.name.toLowerCase(); })[0] ||
                        list.filter(function (c) { return c.image === info.image; })[0];
                    var stop = el.querySelector('[data-action="stop"]');
                    if (hit && hit.restart_only && stop) {
                        stop.disabled = true;
                        stop.lastChild.textContent = 'Stop (restart only for this one)';
                    }
                })
                .catch(function () {});
        }
        el.querySelectorAll('.gc-act').forEach(function (b) {
            b.addEventListener('click', function () {
                var action = b.getAttribute('data-action');
                if (action === 'open') {
                    closeSheet(true);
                    location.href = info.url;
                    return;
                }
                // Stop asks for a second tap instead of another dialog
                if (action === 'stop' && !b.classList.contains('gc-act-confirm')) {
                    b.classList.add('gc-act-confirm');
                    b.lastChild.textContent = 'Tap again to stop ' + info.name;
                    return;
                }
                b.classList.add('gc-act-busy');
                GoodiecatApp.control(info.name, info.image, action);
                closeSheet();
            });
        });
    }

    function button(action, label, disabled) {
        return '<button class="gc-act gc-act-' + action + '" data-action="' + action + '"' + (disabled ? ' disabled' : '') + '>' +
            ICONS[action] + '<span>' + label + '</span></button>';
    }

    // the tile's own link and the rest of the tile both open the panel instead
    document.addEventListener('click', function (e) {
        var tile = e.target.closest && e.target.closest('.docker-container');
        if (!tile || !tile.closest('.widget-type-docker-containers')) return;
        e.preventDefault();
        e.stopPropagation();
        openContainerMenu(tileInfo(tile));
    }, true);
})();
