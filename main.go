package main

import (
	"fmt"
	"log"
	"net/http"
)

// Handler function for the home page
func homePage(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Psilocon Valley</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body {
            margin: 0;
            background: #020617;
            color: #e5e7eb;
            font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
            line-height: 1.6;
        }
        header {
            padding: 2rem 1.5rem;
            text-align: center;
            border-bottom: 1px solid rgba(148,163,184,0.2);
        }
        h1 {
            margin: 0 0 0.5rem;
            font-size: clamp(2rem, 4vw, 3rem);
        }
        .tagline {
            color: #94a3b8;
            max-width: 640px;
            margin: 0 auto;
        }
        main {
            max-width: 960px;
            margin: 0 auto;
            padding: 2rem 1.5rem;
        }
        .card {
            background: rgba(15,23,42,0.6);
            border: 1px solid rgba(148,163,184,0.2);
            border-radius: 12px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
        }
        .card h2 {
            margin-top: 0;
        }
        .btn {
            display: inline-block;
            padding: 0.8rem 1.6rem;
            border-radius: 999px;
            background: linear-gradient(135deg, #6366f1, #8b5cf6);
            color: #f9fafb;
            text-decoration: none;
            font-weight: 500;
        }
        .muted {
            color: #94a3b8;
            font-size: 0.9rem;
        }
        nav a {
            margin: 0 0.75rem;
            color: #cbd5f5;
            text-decoration: none;
        }
        nav a:hover {
            color: #fff;
        }
    </style>
</head>
<body>
<header>
    <h1>Psilocon Valley</h1>
    <p class="tagline">Tools for engineers and founders who need to get into deep work fast.</p>
    <nav>
        <a href="/">Home</a>
        <a href="/trigger-log">Trigger Log</a>
        <a href="/blog">Blog</a>
    </nav>
</header>

<main>
    <div class="card">
        <h2>Trigger Log — Free Flow Timer</h2>
        <p>Click a button. Start a distraction‑free session. Choose 25, 50, or 90 minutes. No login, no noise.</p>
        <a class="btn" href="/trigger-log">Enter Flow</a>
        <p class="muted" style="margin-top:1rem;">Built with Go. Deployed on Railway.</p>
    </div>

    <div class="card">
        <h2>What is this?</h2>
        <p>Psilocon Valley is my lab for building small, useful tools. Trigger Log is the first one.</p>
    </div>
</main>
</body>
</html>`
	fmt.Fprint(w, html)
}

// Handler function for the blog (placeholder for now)
func blogPage(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Blog | Psilocon Valley</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body {
            margin: 0;
            background: #020617;
            color: #e5e7eb;
            font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
            line-height: 1.6;
        }
        header {
            padding: 2rem 1.5rem;
            text-align: center;
            border-bottom: 1px solid rgba(148,163,184,0.2);
        }
        h1 { margin: 0 0 0.5rem; font-size: clamp(2rem, 4vw, 3rem); }
        nav a { margin: 0 0.75rem; color: #cbd5f5; text-decoration: none; }
        nav a:hover { color: #fff; }
        main { max-width: 960px; margin: 0 auto; padding: 2rem 1.5rem; }
        .muted { color: #94a3b8; }
    </style>
</head>
<body>
<header>
    <h1>The Lab Blog</h1>
    <nav>
        <a href="/">Home</a>
        <a href="/trigger-log">Trigger Log</a>
        <a href="/blog">Blog</a>
    </nav>
</header>
<main>
    <p class="muted">Coming soon. Notes on building tools with Go, shipping fast, and staying in deep work.</p>
</main>
</body>
</html>`
	fmt.Fprint(w, html)
}

// Handler function for Trigger Log
func triggerLogPage(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Trigger Log | Psilocon Valley</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="manifest" href="/manifest.json">
    <meta name="theme-color" content="#6366f1">
    <style>
        body {
            margin: 0;
            height: 100vh;
            background: #020617;
            color: #e5e7eb;
            display: flex;
            align-items: center;
            justify-content: center;
            font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        }
        .container { text-align: center; max-width: 560px; padding: 1rem; }
        .title {
            font-size: 1rem;
            letter-spacing: 0.2em;
            text-transform: uppercase;
            color: #64748b;
        }
        .headline {
            font-size: 2rem;
            margin: 0.5rem 0 1rem;
        }
        .subtitle {
            color: #9ca3af;
            margin-bottom: 1.5rem;
        }
        .btn {
            padding: 0.9rem 2.4rem;
            border-radius: 999px;
            border: none;
            background: linear-gradient(135deg, #6366f1, #8b5cf6);
            color: #f9fafb;
            font-size: 0.95rem;
            font-weight: 500;
            cursor: pointer;
        }
        .btn:active { transform: scale(0.98); }
        .btn.secondary {
            background: #334155;
            color: #e5e7eb;
        }
        .timer {
            font-size: 4rem;
            letter-spacing: 0.1em;
            margin-bottom: 1rem;
        }
        .hidden { display: none; }
        .options {
            display: flex;
            gap: 0.5rem;
            justify-content: center;
            margin: 1rem 0 1.5rem;
        }
        .option {
            padding: 0.6rem 1.2rem;
            border-radius: 999px;
            border: 1px solid #334155;
            background: #0b1220;
            color: #cbd5f5;
            cursor: pointer;
        }
        .option.active {
            border-color: #6366f1;
            background: linear-gradient(135deg, #6366f1, #8b5cf6);
            color: #fff;
        }
        .row {
            display: flex;
            gap: 0.75rem;
            justify-content: center;
            flex-wrap: wrap;
            margin-top: 1rem;
        }
        a.back {
            display: inline-block;
            margin-top: 1.5rem;
            color: #94a3b8;
            text-decoration: none;
        }
        a.back:hover { color: #fff; }
    </style>
</head>
<body>
<div class="container">
    <div class="title">TRIGGER LOG</div>
    <h1 class="headline">Zero distractions. Just build.</h1>

    <div id="pre-session">
        <p class="subtitle">Choose your session length, then enter flow.</p>

        <div class="options" role="tablist" aria-label="Session length">
            <button class="option" data-min="25" role="tab" aria-selected="false">25 min</button>
            <button class="option active" data-min="50" role="tab" aria-selected="true">50 min</button>
            <button class="option" data-min="90" role="tab" aria-selected="false">90 min</button>
        </div>

        <button id="startBtn" class="btn">ENTER FLOW</button>
        <div><a class="back" href="/">← Back to Psilocon Valley</a></div>
    </div>

    <div id="session" class="hidden">
        <div id="timer" class="timer">50:00</div>
        <p class="subtitle">You are in the flow. Stay with the work.</p>
        <div class="row">
            <button id="shareBtn" class="btn">Share Session</button>
            <button id="pauseBtn" class="btn secondary">Pause</button>
        </div>
    </div>

    <div id="complete" class="hidden">
        <h2>Flow complete.</h2>
        <p class="subtitle">Ship something before you leave this page.</p>
        <div class="row" id="post-actions">
            <!-- reminder + calendar injected by JS -->
        </div>
        <div><a class="back" href="/">← Back to Psilocon Valley</a></div>
    </div>
</div>

<script>
    const startBtn = document.getElementById("startBtn");
    const preSession = document.getElementById("pre-session");
    const session = document.getElementById("session");
    const complete = document.getElementById("complete");
    const timerDisplay = document.getElementById("timer");
    const optionButtons = Array.from(document.querySelectorAll(".option"));
    const shareBtn = document.getElementById("shareBtn");
    const pauseBtn = document.getElementById("pauseBtn");
    const postActions = document.getElementById("post-actions");

    let selectedMinutes = 50;
    let remainingSeconds = selectedMinutes * 60;
    let intervalId = null;
    let paused = false;

    function formatTime(seconds) {
        const m = Math.floor(seconds / 60);
        const s = seconds % 60;
        const mm = String(m).padStart(2, "0");
        const ss = String(s).padStart(2, "0");
        return mm + ":" + ss;
    }

    function updateTimerDisplay() {
        timerDisplay.textContent = formatTime(Math.max(0, remainingSeconds));
    }

    // timer options
    optionButtons.forEach(btn => {
        btn.addEventListener("click", () => {
            optionButtons.forEach(b => {
                b.classList.remove("active");
                b.setAttribute("aria-selected", "false");
            });
            btn.classList.add("active");
            btn.setAttribute("aria-selected", "true");
            selectedMinutes = parseInt(btn.dataset.min, 10);
            remainingSeconds = selectedMinutes * 60;
            updateTimerDisplay();
        });
    });
    updateTimerDisplay();

    // start session
    startBtn.addEventListener("click", () => {
        preSession.classList.add("hidden");
        session.classList.remove("hidden");
        remainingSeconds = selectedMinutes * 60;
        paused = false;
        pauseBtn.textContent = "Pause";
        updateTimerDisplay();

        clearInterval(intervalId);
        intervalId = setInterval(() => {
            if (paused) return;
            remainingSeconds -= 1;
            if (remainingSeconds <= 0) {
                clearInterval(intervalId);
                session.classList.add("hidden");
                complete.classList.remove("hidden");

                // add post-session actions
                const endTime = Date.now() + 0; // already ended
                const startTime = Date.now() - selectedMinutes * 60 * 1000;
                const startISO = new Date(startTime).toISOString().replace(/[-:]/g,'').split('.')[0] + 'Z';
                const endISO = new Date(Date.now()).toISOString().replace(/[-:]/g,'').split('.')[0] + 'Z';

                postActions.innerHTML = `
                  <button id="remindBtn" class="btn">Remind me when done</button>
                  <a class="btn" href="https://calendar.google.com/calendar/render?action=TEMPLATE&text=Trigger%20Log%20Session&details=${encodeURIComponent(selectedMinutes + ' minutes deep work session via Psilocon Valley')}&dates=${startISO}/${endISO}" target="_blank">Add to Calendar</a>
                `;

                // notification permission + reminder
                if ('Notification' in window && Notification.permission === 'default') {
                  Notification.requestPermission();
                }
                document.getElementById('remindBtn').addEventListener('click', () => {
                  if ('Notification' in window && Notification.permission === 'granted') {
                    new Notification('Trigger Log', { body: 'Your ' + selectedMinutes + '‑minute flow session is complete. Ship something.' });
                  } else {
                    alert('Enable notifications in your browser to get reminders.');
                  }
                });

                return;
            }
            updateTimerDisplay();
        }, 1000);
    });

    // pause/resume
    pauseBtn.addEventListener("click", () => {
        paused = !paused;
        pauseBtn.textContent = paused ? "Resume" : "Pause";
    });

    // share session
    shareBtn.addEventListener("click", async () => {
        const sessionId = (crypto && crypto.randomUUID) ? crypto.randomUUID() : String(Math.random()).slice(2);
        const url = location.origin + '/trigger-log?session=' + sessionId;
        try {
            if (navigator.share) {
                await navigator.share({
                    title: 'Trigger Log Session',
                    text: 'I’m in deep work for ' + selectedMinutes + ' minutes. Do not disturb.',
                    url
                });
            } else {
                await navigator.clipboard.writeText(url);
                alert('Session link copied to clipboard: ' + url);
            }
        } catch (_) {}
    });

    // PWA service worker
    if ('serviceWorker' in navigator) {
        navigator.serviceWorker.register('/sw.js').catch(() => {});
    }
</script>
</body>
</html>`
	fmt.Fprint(w, html)
}

// PWA manifest
func manifestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{
  "name": "Trigger Log — Psilocon Valley",
  "short_name": "Trigger Log",
  "start_url": "/trigger-log",
  "display": "standalone",
  "background_color": "#020617",
  "theme_color": "#6366f1",
  "icons": [
    {
      "src": "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'%3E%3Ccircle cx='50' cy='50' r='50' fill='%236366f1'/%3E%3Ctext x='50' y='58' font-size='42' text-anchor='middle' fill='white' font-family='system-ui'%3E⏱%3C/text%3E%3C/svg%3E",
      "sizes": "any",
      "type": "image/svg+xml"
    }
  ]
}`)
}

// Service worker
func swHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	fmt.Fprint(w, `
self.addEventListener('install', e => e.waitUntil(self.skipWaiting()));
self.addEventListener('activate', e => e.waitUntil(self.clients.claim()));
self.addEventListener('fetch', e => {
  // simple pass-through for MVP
  e.respondWith(fetch(e.request));
});
`)
}

func main() {
	http.HandleFunc("/", homePage)
	http.HandleFunc("/blog", blogPage)
	http.HandleFunc("/trigger-log", triggerLogPage)
	http.HandleFunc("/manifest.json", manifestHandler)
	http.HandleFunc("/sw.js", swHandler)

	fmt.Println("🚀 Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
