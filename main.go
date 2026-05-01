package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

const backgroundImage = "https://images.pexels.com/photos/20133547/pexels-photo-20133547.jpeg"

// Home page
func homePage(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Psiloconvalley</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        :root {
            color-scheme: dark;
            --deep-soil: #02040a;
            --soil: #07111f;
            --wet-bark: #102033;
            --night-blue: #0b1f3a;
            --mushroom-blue: #4fc3ff;
            --cold-glow: #8bdcff;
            --moon: #dcecff;
            --mycelium: #f1efe3;
            --spore: #d8a35d;
            --rust: #8a4f36;
        }

        * {
            box-sizing: border-box;
        }

        body {
            margin: 0;
            min-height: 100vh;
            color: var(--moon);
            font-family: Georgia, "Times New Roman", serif;
            line-height: 1.6;
            background:
                linear-gradient(rgba(2, 4, 10, 0.68), rgba(2, 4, 10, 0.95)),
                radial-gradient(circle at 18% 12%, rgba(79, 195, 255, 0.22), transparent 38%),
                radial-gradient(circle at 85% 82%, rgba(216, 163, 93, 0.13), transparent 42%),
                radial-gradient(circle at 50% 110%, rgba(139, 220, 255, 0.10), transparent 48%),
                url("` + backgroundImage + `");
            background-size: cover;
            background-position: center;
            background-attachment: fixed;
            overflow-x: hidden;
        }

        body::before {
            content: "";
            position: fixed;
            inset: 0;
            pointer-events: none;
            background-image:
                radial-gradient(circle at 20% 30%, rgba(220, 236, 255, 0.13) 0 1px, transparent 2px),
                radial-gradient(circle at 70% 60%, rgba(79, 195, 255, 0.14) 0 1px, transparent 2px),
                radial-gradient(circle at 40% 80%, rgba(216, 163, 93, 0.10) 0 1px, transparent 2px);
            background-size: 90px 90px, 140px 140px, 190px 190px;
            opacity: 0.72;
            mix-blend-mode: screen;
            animation: driftField 22s linear infinite;
            z-index: 0;
        }

        @keyframes driftField {
            from {
                transform: translate3d(0, 0, 0);
            }
            to {
                transform: translate3d(-50px, 35px, 0);
            }
        }

        .cursor-glow {
            position: fixed;
            left: 50%;
            top: 50%;
            width: 320px;
            height: 320px;
            border-radius: 999px;
            pointer-events: none;
            transform: translate(-50%, -50%);
            background:
                radial-gradient(circle, rgba(79, 195, 255, 0.18), transparent 62%),
                radial-gradient(circle, rgba(216, 163, 93, 0.08), transparent 72%);
            mix-blend-mode: screen;
            opacity: 0;
            transition: opacity 0.25s ease;
            z-index: 1;
        }

        .spore-layer {
            position: fixed;
            inset: 0;
            overflow: hidden;
            pointer-events: none;
            z-index: 1;
        }

        .spore-layer span {
            position: absolute;
            left: var(--x);
            top: 110vh;
            width: var(--size);
            height: var(--size);
            border-radius: 999px;
            background: rgba(220, 236, 255, 0.46);
            box-shadow:
                0 0 10px rgba(79, 195, 255, 0.42),
                0 0 20px rgba(216, 163, 93, 0.16);
            animation: floatSpore var(--duration) linear infinite;
            animation-delay: var(--delay);
            opacity: 0;
        }

        @keyframes floatSpore {
            0% {
                transform: translate3d(0, 0, 0);
                opacity: 0;
            }
            10% {
                opacity: 0.75;
            }
            80% {
                opacity: 0.42;
            }
            100% {
                transform: translate3d(var(--drift), -125vh, 0);
                opacity: 0;
            }
        }

        header {
            padding: 2rem 1.5rem 1.5rem;
            text-align: center;
            border-bottom: 1px solid rgba(139, 220, 255, 0.20);
            background: rgba(2, 4, 10, 0.72);
            backdrop-filter: blur(16px);
            position: sticky;
            top: 0;
            z-index: 5;
            box-shadow:
                0 18px 50px rgba(0, 0, 0, 0.55),
                0 0 35px rgba(79, 195, 255, 0.08);
        }

        h1 {
            margin: 0 0 0.5rem;
            font-size: clamp(2.2rem, 5vw, 4rem);
            letter-spacing: 0.12em;
            text-transform: uppercase;
            color: var(--mycelium);
            text-shadow:
                0 0 24px rgba(79, 195, 255, 0.46),
                0 0 58px rgba(139, 220, 255, 0.24);
        }

        .tagline {
            color: #bfd4e8;
            max-width: 720px;
            margin: 0.25rem auto 1rem;
            font-size: 1rem;
        }

        nav a {
            margin: 0 0.75rem;
            color: var(--cold-glow);
            text-decoration: none;
            font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
            font-size: 0.82rem;
            text-transform: uppercase;
            letter-spacing: 0.16em;
        }

        nav a:hover {
            color: var(--spore);
        }

        main {
            max-width: 980px;
            margin: 0 auto;
            padding: 3rem 1.5rem 4rem;
            position: relative;
            z-index: 2;
        }

        .hero-note {
            max-width: 740px;
            margin: 0 auto 2rem;
            text-align: center;
            color: #c5d4de;
            font-size: 1.05rem;
        }

        .card {
            position: relative;
            background:
                linear-gradient(145deg, rgba(5, 12, 22, 0.88), rgba(2, 4, 10, 0.92)),
                radial-gradient(circle at top left, rgba(79, 195, 255, 0.16), transparent 42%);
            border: 1px solid rgba(139, 220, 255, 0.27);
            border-radius: 26px;
            padding: 1.9rem 1.7rem;
            margin-bottom: 1.7rem;
            overflow: hidden;
            box-shadow:
                0 34px 90px rgba(0, 0, 0, 0.72),
                0 0 40px rgba(79, 195, 255, 0.10),
                inset 0 0 55px rgba(216, 163, 93, 0.04);
            transform-style: preserve-3d;
            transition: transform 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
        }

        .card:hover {
            border-color: rgba(216, 163, 93, 0.44);
            box-shadow:
                0 42px 110px rgba(0, 0, 0, 0.82),
                0 0 55px rgba(79, 195, 255, 0.18),
                inset 0 0 70px rgba(216, 163, 93, 0.06);
        }

        .card::before {
            content: "";
            position: absolute;
            inset: -40%;
            background:
                radial-gradient(circle at 20% 30%, rgba(220, 236, 255, 0.11), transparent 35%),
                radial-gradient(circle at 70% 80%, rgba(216, 163, 93, 0.10), transparent 35%);
            opacity: 0.88;
            pointer-events: none;
            mix-blend-mode: screen;
        }

        .card h2,
        .card p,
        .card a,
        .small-signal {
            position: relative;
            z-index: 1;
        }

        .card h2 {
            margin-top: 0;
            color: var(--mycelium);
            font-size: 1.6rem;
        }

        .card p {
            color: #c5d4de;
        }

        .small-signal {
            display: inline-block;
            margin-bottom: 0.8rem;
            color: var(--cold-glow);
            font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.22em;
        }

        .btn {
            display: inline-block;
            padding: 0.85rem 1.9rem;
            border-radius: 999px;
            background: linear-gradient(135deg, var(--spore), var(--cold-glow), var(--mushroom-blue));
            color: #02040a;
            text-decoration: none;
            font-weight: 800;
            border: none;
            box-shadow:
                0 20px 55px rgba(79, 195, 255, 0.28),
                0 0 34px rgba(216, 163, 93, 0.16);
            transition: transform 0.12s ease, filter 0.12s ease, box-shadow 0.12s ease;
            font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        }

        .btn:hover {
            transform: translateY(-2px);
            filter: brightness(1.06);
            box-shadow:
                0 26px 70px rgba(79, 195, 255, 0.36),
                0 0 50px rgba(216, 163, 93, 0.22);
        }

        .muted {
            color: #8598a8;
            font-size: 0.9rem;
        }
    </style>
</head>
<body>
<div id="cursorGlow" class="cursor-glow"></div>
<div id="sporeLayer" class="spore-layer"></div>

<header>
    <h1>Psiloconvalley</h1>
    <p class="tagline">A living lab of small tools for focus, clarity, and protected creative time.</p>
    <nav>
        <a href="/">Home</a>
        <a href="/trigger-log">Trigger Log</a>
        <a href="/blog">Blog</a>
    </nav>
</header>

<main>
    <p class="hero-note">
        Built from the forest floor up: quiet tools for anyone trying to protect attention and do meaningful work.
    </p>

    <div class="card interactive-card">
        <span class="small-signal">First Tool</span>
        <h2>Trigger Log — Free Focus Timer</h2>
        <p>
            Choose 25, 50, or 90 minutes. Set your intention. Enter a protected focus session.
            No login, no dashboard, no tracking — just a clean ritual for getting started.
        </p>
        <a class="btn" href="/trigger-log">Enter Flow</a>
        <p class="muted" style="margin-top:1rem;">Built with Go. Deployed on Railway.</p>
    </div>

    <div class="card interactive-card">
        <span class="small-signal">What is this?</span>
        <h2>A small underground lab</h2>
        <p>
            Psiloconvalley is where I build useful experiments in public.
            Trigger Log is the first release: a simple tool for protecting attention.
        </p>
    </div>
</main>

<script>
    const cards = document.querySelectorAll(".interactive-card");
    const cursorGlow = document.getElementById("cursorGlow");
    const sporeLayer = document.getElementById("sporeLayer");

    window.addEventListener("mousemove", (event) => {
        if (!cursorGlow) return;
        cursorGlow.style.left = event.clientX + "px";
        cursorGlow.style.top = event.clientY + "px";
        cursorGlow.style.opacity = "1";
    });

    window.addEventListener("mouseleave", () => {
        if (!cursorGlow) return;
        cursorGlow.style.opacity = "0";
    });

    cards.forEach((card) => {
        card.addEventListener("mousemove", (event) => {
            const rect = card.getBoundingClientRect();
            const x = event.clientX - rect.left;
            const y = event.clientY - rect.top;

            const rotateY = ((x / rect.width) - 0.5) * 8;
            const rotateX = ((y / rect.height) - 0.5) * -8;

            card.style.transform = "rotateX(" + rotateX + "deg) rotateY(" + rotateY + "deg) translateY(-3px)";
        });

        card.addEventListener("mouseleave", () => {
            card.style.transform = "rotateX(0deg) rotateY(0deg) translateY(0)";
        });
    });

    if (sporeLayer) {
        for (let i = 0; i < 28; i++) {
            const spore = document.createElement("span");
            spore.style.setProperty("--x", Math.random() * 100 + "vw");
            spore.style.setProperty("--size", (Math.random() * 3 + 2) + "px");
            spore.style.setProperty("--duration", (Math.random() * 13 + 15) + "s");
            spore.style.setProperty("--delay", (Math.random() * -22) + "s");
            spore.style.setProperty("--drift", (Math.random() * 140 - 70) + "px");
            sporeLayer.appendChild(spore);
        }
    }
</script>
</body>
</html>`
	fmt.Fprint(w, html)
}

// Blog page
func blogPage(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Blog | Psiloconvalley</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        :root {
            color-scheme: dark;
            --deep-soil: #02040a;
            --mushroom-blue: #4fc3ff;
            --cold-glow: #8bdcff;
            --moon: #dcecff;
            --mycelium: #f1efe3;
            --spore: #d8a35d;
        }

        body {
            margin: 0;
            min-height: 100vh;
            color: var(--moon);
            font-family: Georgia, "Times New Roman", serif;
            line-height: 1.6;
            background:
                linear-gradient(rgba(2, 4, 10, 0.76), rgba(2, 4, 10, 0.96)),
                radial-gradient(circle at 18% 12%, rgba(79, 195, 255, 0.22), transparent 38%),
                url("` + backgroundImage + `");
            background-size: cover;
            background-position: center;
            background-attachment: fixed;
        }

        header {
            padding: 2rem 1.5rem;
            text-align: center;
            border-bottom: 1px solid rgba(139, 220, 255, 0.20);
            background: rgba(2, 4, 10, 0.72);
            backdrop-filter: blur(16px);
        }

        h1 {
            margin: 0 0 0.5rem;
            font-size: clamp(2rem, 4vw, 3rem);
            color: var(--mycelium);
            text-shadow: 0 0 24px rgba(79, 195, 255, 0.4);
        }

        nav a {
            margin: 0 0.75rem;
            color: var(--cold-glow);
            text-decoration: none;
            text-transform: uppercase;
            letter-spacing: 0.16em;
            font-size: 0.82rem;
            font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        }

        nav a:hover {
            color: var(--spore);
        }

        main {
            max-width: 960px;
            margin: 0 auto;
            padding: 2rem 1.5rem;
        }

        .panel {
            background: rgba(5, 12, 22, 0.82);
            border: 1px solid rgba(139, 220, 255, 0.24);
            border-radius: 24px;
            padding: 1.8rem;
            box-shadow: 0 30px 80px rgba(0, 0, 0, 0.65);
        }

        .muted {
            color: #c5d4de;
        }
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
    <div class="panel">
        <p class="muted">Coming soon. Notes on building tools with Go, shipping fast, and protecting focus.</p>
    </div>
</main>
</body>
</html>`
	fmt.Fprint(w, html)
}

// Trigger Log page
func triggerLogPage(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Trigger Log | Psiloconvalley</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        :root {
            color-scheme: dark;
            --deep-soil: #02040a;
            --soil: #07111f;
            --wet-bark: #102033;
            --night-blue: #0b1f3a;
            --mushroom-blue: #4fc3ff;
            --cold-glow: #8bdcff;
            --moon: #dcecff;
            --mycelium: #f1efe3;
            --spore: #d8a35d;
            --rust: #8a4f36;
        }

        * {
            box-sizing: border-box;
        }

        body {
            margin: 0;
            min-height: 100vh;
            color: var(--moon);
            display: flex;
            align-items: center;
            justify-content: center;
            font-family: Georgia, "Times New Roman", serif;
            background:
                linear-gradient(rgba(2, 4, 10, 0.70), rgba(2, 4, 10, 0.96)),
                radial-gradient(circle at 18% 12%, rgba(79, 195, 255, 0.24), transparent 38%),
                radial-gradient(circle at 85% 82%, rgba(216, 163, 93, 0.13), transparent 42%),
                radial-gradient(circle at 50% 110%, rgba(139, 220, 255, 0.10), transparent 48%),
                url("` + backgroundImage + `");
            background-size: cover;
            background-position: center;
            background-attachment: fixed;
            padding: 1.2rem;
            overflow-x: hidden;
        }

        body::before {
            content: "";
            position: fixed;
            inset: 0;
            pointer-events: none;
            background-image:
                radial-gradient(circle at 15% 20%, rgba(220, 236, 255, 0.14) 0 1px, transparent 2px),
                radial-gradient(circle at 65% 45%, rgba(79, 195, 255, 0.13) 0 1px, transparent 2px),
                radial-gradient(circle at 40% 85%, rgba(216, 163, 93, 0.10) 0 1px, transparent 2px);
            background-size: 80px 80px, 130px 130px, 190px 190px;
            mix-blend-mode: screen;
            opacity: 0.72;
            animation: drift 18s linear infinite;
            z-index: 0;
        }

        @keyframes drift {
            from { transform: translate3d(0, 0, 0); }
            to { transform: translate3d(-40px, 30px, 0); }
        }

        .cursor-glow {
            position: fixed;
            left: 50%;
            top: 50%;
            width: 320px;
            height: 320px;
            border-radius: 999px;
            pointer-events: none;
            transform: translate(-50%, -50%);
            background:
                radial-gradient(circle, rgba(79, 195, 255, 0.20), transparent 62%),
                radial-gradient(circle, rgba(216, 163, 93, 0.08), transparent 72%);
            mix-blend-mode: screen;
            opacity: 0;
            transition: opacity 0.25s ease;
            z-index: 1;
        }

        .spore-layer {
            position: fixed;
            inset: 0;
            overflow: hidden;
            pointer-events: none;
            z-index: 1;
        }

        .spore-layer span {
            position: absolute;
            left: var(--x);
            top: 110vh;
            width: var(--size);
            height: var(--size);
            border-radius: 999px;
            background: rgba(220, 236, 255, 0.46);
            box-shadow:
                0 0 10px rgba(79, 195, 255, 0.42),
                0 0 20px rgba(216, 163, 93, 0.16);
            animation: floatSpore var(--duration) linear infinite;
            animation-delay: var(--delay);
            opacity: 0;
        }

        @keyframes floatSpore {
            0% {
                transform: translate3d(0, 0, 0);
                opacity: 0;
            }
            10% {
                opacity: 0.75;
            }
            80% {
                opacity: 0.42;
            }
            100% {
                transform: translate3d(var(--drift), -125vh, 0);
                opacity: 0;
            }
        }

        .container {
            text-align: center;
            width: 100%;
            max-width: 640px;
            padding: 1.8rem;
            border-radius: 30px;
            background:
                linear-gradient(145deg, rgba(5, 12, 22, 0.90), rgba(2, 4, 10, 0.94)),
                radial-gradient(circle at top, rgba(79, 195, 255, 0.18), transparent 44%);
            border: 1px solid rgba(139, 220, 255, 0.30);
            box-shadow:
                0 40px 120px rgba(0, 0, 0, 0.82),
                0 0 70px rgba(79, 195, 255, 0.16),
                inset 0 0 70px rgba(216, 163, 93, 0.04);
            position: relative;
            overflow: hidden;
            transition: transform 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
            z-index: 2;
        }

        .container:hover {
            transform: translateY(-3px);
            border-color: rgba(216, 163, 93, 0.42);
            box-shadow:
                0 48px 135px rgba(0, 0, 0, 0.88),
                0 0 88px rgba(79, 195, 255, 0.22),
                inset 0 0 80px rgba(216, 163, 93, 0.06);
        }

        .container::before {
            content: "";
            position: absolute;
            inset: -45%;
            background:
                radial-gradient(circle at 20% 25%, rgba(220, 236, 255, 0.12), transparent 35%),
                radial-gradient(circle at 75% 75%, rgba(216, 163, 93, 0.10), transparent 35%),
                radial-gradient(circle at 55% 55%, rgba(79, 195, 255, 0.10), transparent 40%);
            mix-blend-mode: screen;
            opacity: 0.9;
            pointer-events: none;
            animation: pulseField 9s ease-in-out infinite;
        }

        @keyframes pulseField {
            0%, 100% {
                opacity: 0.55;
                transform: scale(1);
            }
            50% {
                opacity: 0.95;
                transform: scale(1.05);
            }
        }

        .title,
        .headline,
        .subtitle,
        .options,
        .row,
        .timer,
        a.back,
        .intent-wrap,
        #intentDisplay,
        .progress-track {
            position: relative;
            z-index: 1;
        }

        .title {
            font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
            font-size: 0.78rem;
            letter-spacing: 0.24em;
            text-transform: uppercase;
            color: var(--cold-glow);
            margin-bottom: 0.6rem;
        }

        .headline {
            font-size: clamp(2rem, 7vw, 3.5rem);
            line-height: 1;
            margin: 0.4rem 0 1rem;
            color: var(--mycelium);
            text-shadow:
                0 0 24px rgba(79, 195, 255, 0.46),
                0 0 58px rgba(139, 220, 255, 0.24);
        }

        .subtitle {
            color: #c5d4de;
            margin-bottom: 1.4rem;
            font-size: 1rem;
        }

        .options {
            display: flex;
            gap: 0.55rem;
            justify-content: center;
            margin: 1rem 0 1.3rem;
            flex-wrap: wrap;
        }

        .option {
            padding: 0.65rem 1.25rem;
            border-radius: 999px;
            border: 1px solid rgba(139, 220, 255, 0.24);
            background: rgba(2, 4, 10, 0.74);
            color: #c5d4de;
            cursor: pointer;
            font-size: 0.86rem;
            font-weight: 700;
            transition: transform 0.12s ease, border-color 0.12s ease, background 0.12s ease;
            font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        }

        .option:hover {
            transform: translateY(-1px);
            border-color: rgba(216, 163, 93, 0.46);
        }

        .option.active {
            border-color: rgba(139, 220, 255, 0.82);
            background: linear-gradient(135deg, var(--spore), var(--cold-glow), var(--mushroom-blue));
            color: #02040a;
            box-shadow:
                0 12px 30px rgba(79, 195, 255, 0.26),
                0 0 34px rgba(216, 163, 93, 0.14);
        }

        .intent-wrap {
            margin-bottom: 1rem;
        }

        #intentInput {
            width: 100%;
            max-width: 410px;
            padding: 0.85rem 1rem;
            border-radius: 999px;
            border: 1px solid rgba(139, 220, 255, 0.26);
            background: rgba(2, 4, 10, 0.74);
            color: var(--mycelium);
            font-size: 0.95rem;
            outline: none;
            text-align: center;
        }

        #intentInput:focus {
            border-color: rgba(139, 220, 255, 0.75);
            box-shadow: 0 0 34px rgba(79, 195, 255, 0.18);
        }

        .btn {
            padding: 0.9rem 2.4rem;
            border-radius: 999px;
            border: none;
            background: linear-gradient(135deg, var(--spore), var(--cold-glow), var(--mushroom-blue));
            color: #02040a;
            font-size: 0.95rem;
            font-weight: 800;
            cursor: pointer;
            position: relative;
            z-index: 1;
            box-shadow:
                0 20px 55px rgba(79, 195, 255, 0.30),
                0 0 38px rgba(216, 163, 93, 0.16);
            transition: transform 0.12s ease, filter 0.12s ease, box-shadow 0.12s ease;
            font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        }

        .btn:hover {
            transform: translateY(-2px);
            filter: brightness(1.07);
        }

        .btn:active {
            transform: scale(0.98);
        }

        .btn.secondary {
            background: rgba(2, 4, 10, 0.76);
            color: var(--mycelium);
            border: 1px solid rgba(139, 220, 255, 0.30);
            box-shadow: none;
        }

        .timer {
            font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
            font-size: clamp(3.8rem, 16vw, 7rem);
            font-weight: 900;
            letter-spacing: 0.05em;
            margin: 1rem 0 0.8rem;
            color: var(--mycelium);
        }

        .timer.breathing {
            animation: timerGlow 2.8s ease-in-out infinite;
        }

        @keyframes timerGlow {
            0%, 100% {
                text-shadow:
                    0 0 8px rgba(79, 195, 255, 0.28),
                    0 0 20px rgba(216, 163, 93, 0.10);
            }
            50% {
                text-shadow:
                    0 0 28px rgba(79, 195, 255, 0.92),
                    0 0 62px rgba(139, 220, 255, 0.42);
            }
        }

        .progress-track {
            width: 100%;
            max-width: 440px;
            height: 10px;
            margin: 0 auto 1.2rem;
            border-radius: 999px;
            overflow: hidden;
            background: rgba(2, 4, 10, 0.82);
            border: 1px solid rgba(139, 220, 255, 0.20);
        }

        .progress-bar {
            width: 0%;
            height: 100%;
            border-radius: inherit;
            background: linear-gradient(90deg, var(--spore), var(--cold-glow), var(--mushroom-blue));
            box-shadow:
                0 0 18px rgba(79, 195, 255, 0.52),
                0 0 30px rgba(216, 163, 93, 0.20);
            transition: width 0.35s linear;
        }

        #intentDisplay {
            color: var(--cold-glow);
            font-style: italic;
            margin: -0.5rem 0 1.2rem;
        }

        .hidden {
            display: none;
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
            color: var(--cold-glow);
            text-decoration: none;
            font-size: 0.9rem;
        }

        a.back:hover {
            color: var(--spore);
        }
    </style>
</head>
<body>
<div id="cursorGlow" class="cursor-glow"></div>
<div id="sporeLayer" class="spore-layer"></div>

<div class="container">
    <div class="title">TRIGGER LOG</div>
    <h1 class="headline">Protect the work.</h1>

    <div id="pre-session">
        <p class="subtitle">Choose a session length. Set an intention. Enter flow.</p>

        <div class="options" role="tablist" aria-label="Session length">
            <button class="option" data-min="25" role="tab" aria-selected="false">25 min</button>
            <button class="option active" data-min="50" role="tab" aria-selected="true">50 min</button>
            <button class="option" data-min="90" role="tab" aria-selected="false">90 min</button>
        </div>

        <div class="intent-wrap">
            <input id="intentInput" type="text" placeholder="What are you focusing on?">
        </div>

        <button id="startBtn" class="btn">ENTER FLOW</button>
        <div><a class="back" href="/">← Back to Psiloconvalley</a></div>
    </div>

    <div id="session" class="hidden">
        <div id="timer" class="timer">50:00</div>
        <div class="progress-track">
            <div id="progressBar" class="progress-bar"></div>
        </div>
        <p class="subtitle">Stay with the work. Let the noise pass.</p>
        <p id="intentDisplay"></p>

        <div class="row">
            <button id="shareBtn" class="btn">Share Session</button>
            <button id="pauseBtn" class="btn secondary">Pause</button>
        </div>
    </div>

    <div id="complete" class="hidden">
        <h2 class="headline">Session complete.</h2>
        <p class="subtitle">Capture the output before the signal fades.</p>
        <div class="row" id="post-actions"></div>
        <div><a class="back" href="/">← Back to Psiloconvalley</a></div>
    </div>
</div>

<script>
    const startBtn = document.getElementById("startBtn");
    const preSession = document.getElementById("pre-session");
    const session = document.getElementById("session");
    const complete = document.getElementById("complete");
    const timerDisplay = document.getElementById("timer");
    const progressBar = document.getElementById("progressBar");
    const optionButtons = Array.from(document.querySelectorAll(".option"));
    const shareBtn = document.getElementById("shareBtn");
    const pauseBtn = document.getElementById("pauseBtn");
    const postActions = document.getElementById("post-actions");
    const intentInput = document.getElementById("intentInput");
    const intentDisplay = document.getElementById("intentDisplay");
    const cursorGlow = document.getElementById("cursorGlow");
    const sporeLayer = document.getElementById("sporeLayer");

    let selectedMinutes = 50;
    let totalSeconds = selectedMinutes * 60;
    let remainingSeconds = selectedMinutes * 60;
    let intervalId = null;
    let paused = false;
    let currentIntent = "";

    window.addEventListener("mousemove", (event) => {
        if (!cursorGlow) return;
        cursorGlow.style.left = event.clientX + "px";
        cursorGlow.style.top = event.clientY + "px";
        cursorGlow.style.opacity = "1";
    });

    window.addEventListener("mouseleave", () => {
        if (!cursorGlow) return;
        cursorGlow.style.opacity = "0";
    });

    if (sporeLayer) {
        for (let i = 0; i < 28; i++) {
            const spore = document.createElement("span");
            spore.style.setProperty("--x", Math.random() * 100 + "vw");
            spore.style.setProperty("--size", (Math.random() * 3 + 2) + "px");
            spore.style.setProperty("--duration", (Math.random() * 13 + 15) + "s");
            spore.style.setProperty("--delay", (Math.random() * -22) + "s");
            spore.style.setProperty("--drift", (Math.random() * 140 - 70) + "px");
            sporeLayer.appendChild(spore);
        }
    }

    function formatTime(seconds) {
        const m = Math.floor(seconds / 60);
        const s = seconds % 60;
        const mm = String(m).padStart(2, "0");
        const ss = String(s).padStart(2, "0");
        return mm + ":" + ss;
    }

    function updateTimerDisplay() {
        timerDisplay.textContent = formatTime(Math.max(0, remainingSeconds));

        if (progressBar && totalSeconds > 0) {
            const elapsed = totalSeconds - remainingSeconds;
            const percent = Math.min(100, Math.max(0, (elapsed / totalSeconds) * 100));
            progressBar.style.width = percent + "%";
        }
    }

    optionButtons.forEach(btn => {
        btn.addEventListener("click", () => {
            optionButtons.forEach(b => {
                b.classList.remove("active");
                b.setAttribute("aria-selected", "false");
            });

            btn.classList.add("active");
            btn.setAttribute("aria-selected", "true");

            selectedMinutes = parseInt(btn.dataset.min, 10);
            totalSeconds = selectedMinutes * 60;
            remainingSeconds = selectedMinutes * 60;
            updateTimerDisplay();
        });
    });

    updateTimerDisplay();

    startBtn.addEventListener("click", () => {
        currentIntent = intentInput.value.trim();

        preSession.classList.add("hidden");
        session.classList.remove("hidden");
        complete.classList.add("hidden");

        totalSeconds = selectedMinutes * 60;
        remainingSeconds = selectedMinutes * 60;
        paused = false;
        pauseBtn.textContent = "Pause";
        updateTimerDisplay();

        if (currentIntent.length > 0) {
            intentDisplay.textContent = "Focus: " + currentIntent;
        } else {
            intentDisplay.textContent = "";
        }

        timerDisplay.classList.add("breathing");

        clearInterval(intervalId);

        intervalId = setInterval(() => {
            if (paused) return;

            remainingSeconds -= 1;

            if (remainingSeconds <= 0) {
                clearInterval(intervalId);
                timerDisplay.classList.remove("breathing");
                progressBar.style.width = "100%";
                session.classList.add("hidden");
                complete.classList.remove("hidden");
                postActions.innerHTML = "";
                return;
            }

            updateTimerDisplay();
        }, 1000);
    });

    pauseBtn.addEventListener("click", () => {
        paused = !paused;
        pauseBtn.textContent = paused ? "Resume" : "Pause";

        if (paused) {
            timerDisplay.classList.remove("breathing");
        } else {
            timerDisplay.classList.add("breathing");
        }
    });

    shareBtn.addEventListener("click", async () => {
        const url = location.origin + "/trigger-log";
        let text = "I am in a " + selectedMinutes + "-minute focus session.";

        if (currentIntent.length > 0) {
            text = text + " Focus: " + currentIntent;
        }

        try {
            if (navigator.share) {
                await navigator.share({
                    title: "Trigger Log Session",
                    text: text,
                    url: url
                });
            } else {
                await navigator.clipboard.writeText(url);
                alert("Link copied: " + url);
            }
        } catch (_) {}
    });
</script>
</body>
</html>`
	fmt.Fprint(w, html)
}

func main() {
	http.HandleFunc("/", homePage)
	http.HandleFunc("/blog", blogPage)
	http.HandleFunc("/trigger-log", triggerLogPage)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("🚀 Server starting on port " + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
