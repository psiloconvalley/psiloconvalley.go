(function () {
    "use strict";
    function initNav() {
        var nav = document.querySelector(".site-nav");
        if (!nav) return;
        var btn = nav.querySelector(".nav-toggle");
        var links = nav.querySelector(".nav-links");
        if (!btn || !links) return;

        function closeMenu() {
            links.classList.remove("open");
            btn.setAttribute("aria-expanded", "false");
        }
        function toggleMenu() {
            var isOpen = links.classList.toggle("open");
            btn.setAttribute("aria-expanded", isOpen ? "true" : "false");
        }

        btn.addEventListener("click", function (e) {
            e.stopPropagation();
            toggleMenu();
        });

        links.querySelectorAll("a").forEach(function (link) {
            link.addEventListener("click", function () {
                if (window.innerWidth <= 768) closeMenu();
            });
        });

        document.addEventListener("click", function (e) {
            if (!nav.contains(e.target)) closeMenu();
        });

        window.addEventListener("resize", function () {
            if (window.innerWidth > 768) closeMenu();
        });
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", initNav);
    } else {
        initNav();
    }
})();
