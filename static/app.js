/* Shared animations and interactions for all pages. */
(function () {
    'use strict';

    // Global mouse / scroll state used for all parallax.
    let mouseX = 0, mouseY = 0, scrollY = 0;
    let smoothMouseX = 0, smoothMouseY = 0, smoothScrollY = 0;

    document.addEventListener('mousemove', (e) => {
        mouseX = (e.clientX / window.innerWidth - 0.5) * 2;
        mouseY = (e.clientY / window.innerHeight - 0.5) * 2;
    });
    window.addEventListener('scroll', () => {
        scrollY = window.scrollY;
    }, { passive: true });

    // Smooth interpolation so the parallax never stops abruptly.
    // Lower factor = slower/more subtle follow, higher = snappier.
    const lerpFactor = 0.04;

    function smoothInput() {
        smoothMouseX += (mouseX - smoothMouseX) * lerpFactor;
        smoothMouseY += (mouseY - smoothMouseY) * lerpFactor;
        smoothScrollY += (scrollY - smoothScrollY) * lerpFactor;
    }

    // Parallax on mouse move and scroll for elements with .parallax
    const parallaxElements = document.querySelectorAll('.parallax');
    if (parallaxElements.length) {
        const layers = Array.from(parallaxElements).map((el) => ({
            el,
            speed: parseFloat(el.dataset.speed) || 0.03,
        }));
        function animateParallax() {
            requestAnimationFrame(animateParallax);
            layers.forEach((layer) => {
                const moveX = smoothMouseX * layer.speed * 30;
                const moveY = smoothMouseY * layer.speed * 30 + smoothScrollY * layer.speed * 0.2;
                layer.el.style.transform = `translate(${moveX}px, ${moveY}px)`;
            });
        }
        animateParallax();
    }

    // Scroll reveal for .reveal elements
    const revealElements = document.querySelectorAll('.reveal');
    if (revealElements.length && 'IntersectionObserver' in window) {
        const observer = new IntersectionObserver((entries) => {
            entries.forEach((entry) => {
                if (entry.isIntersecting) {
                    entry.target.classList.add('visible');
                    observer.unobserve(entry.target);
                }
            });
        }, { threshold: 0.1, rootMargin: '0px 0px -50px 0px' });
        revealElements.forEach((el) => observer.observe(el));
    } else if (revealElements.length) {
        revealElements.forEach((el) => el.classList.add('visible'));
    }

    // Add stagger delay to children with .stagger class
    document.querySelectorAll('.stagger').forEach((parent) => {
        parent.querySelectorAll('.stagger-item').forEach((child, i) => {
            child.style.setProperty('--stagger-delay', `${i * 0.08}s`);
        });
    });

    // Ripple effect for buttons (delegated, so dynamically added buttons also ripple)
    document.addEventListener('click', function (e) {
        const btn = e.target.closest('.btn');
        if (!btn) return;
        const rect = btn.getBoundingClientRect();
        const ripple = document.createElement('span');
        ripple.className = 'ripple';
        ripple.style.left = `${e.clientX - rect.left}px`;
        ripple.style.top = `${e.clientY - rect.top}px`;
        btn.appendChild(ripple);
        setTimeout(() => ripple.remove(), 600);
    });

    // Starfield background
    const canvas = document.getElementById('stars');
    if (canvas && canvas.getContext) {
        const ctx = canvas.getContext('2d');
        let width, height;
        let stars = [];
        let shootingStars = [];

        function resize() {
            width = window.innerWidth;
            height = window.innerHeight;
            const dpr = Math.min(window.devicePixelRatio || 1, 2);
            // Full-resolution canvas (up to 2x) for crisp stars
            canvas.width = Math.floor(width * dpr);
            canvas.height = Math.floor(height * dpr);
            ctx.setTransform(1, 0, 0, 1, 0, 0);
            ctx.scale(dpr, dpr);

            const density = 3500;
            const count = Math.min(700, Math.floor((width * height) / density));
            stars = [];
            for (let i = 0; i < count; i++) {
                const depth = Math.random() * 0.8 + 0.2;
                const isBig = Math.random() < 0.12;
                const far = depth < 0.35;
                stars.push({
                    x: Math.random() * width,
                    y: Math.random() * height,
                    baseX: Math.random() * width,
                    baseY: Math.random() * height,
                    radius: isBig ? Math.random() * 1.6 + 1.2 : (far ? Math.random() * 0.7 + 0.2 : Math.random() * 1.1 + 0.4),
                    baseAlpha: isBig ? Math.random() * 0.2 + 0.75 : (far ? Math.random() * 0.25 + 0.3 : Math.random() * 0.35 + 0.55),
                    twinkleSpeed: Math.random() * 0.05 + 0.005,
                    twinklePhase: Math.random() * Math.PI * 2,
                    depth: depth
                });
            }
        }

        function drawStar(s) {
            const parallaxX = smoothMouseX * s.depth * 20;
            const parallaxY = smoothMouseY * s.depth * 20 + smoothScrollY * s.depth * 0.2;
            const x = (s.baseX + parallaxX + width) % width;
            const y = (s.baseY + parallaxY + height) % height;

            s.twinklePhase += s.twinkleSpeed;
            const alpha = s.baseAlpha + Math.sin(s.twinklePhase) * 0.2;
            const finalAlpha = Math.max(0.05, Math.min(1, alpha));

            // Soft halo
            ctx.beginPath();
            ctx.arc(x, y, s.radius * 3, 0, Math.PI * 2);
            ctx.fillStyle = `rgba(255, 255, 255, ${finalAlpha * 0.1})`;
            ctx.fill();
            // Bright core
            ctx.beginPath();
            ctx.arc(x, y, s.radius, 0, Math.PI * 2);
            ctx.fillStyle = `rgba(255, 255, 255, ${finalAlpha})`;
            ctx.fill();
        }

        function spawnShootingStar() {
            if (Math.random() > 0.008) return;
            const startY = Math.random() * (height * 0.5);
            const speed = Math.random() * 10 + 3;
            shootingStars.push({
                x: Math.random() * width * 0.5,
                y: startY,
                vx: speed,
                vy: speed * 0.35,
                length: Math.random() * 90 + 60,
                life: 1.0,
                decay: 0.012
            });
        }

        function drawShootingStar(s) {
            const tailX = s.x - s.vx * (s.length / 5);
            const tailY = s.y - s.vy * (s.length / 5);
            const grad = ctx.createLinearGradient(tailX, tailY, s.x, s.y);
            grad.addColorStop(0, 'rgba(255,255,255,0)');
            grad.addColorStop(0.5, `rgba(255,255,255,${s.life * 0.5})`);
            grad.addColorStop(1, `rgba(255,255,255,${s.life})`);
            ctx.strokeStyle = grad;
            ctx.lineWidth = 2;
            ctx.lineCap = 'round';
            ctx.beginPath();
            ctx.moveTo(tailX, tailY);
            ctx.lineTo(s.x, s.y);
            ctx.stroke();
        }

        let lastFrame = 0;
        const frameInterval = 16; // ~60 fps for smooth animation
        function animate(timestamp) {
            requestAnimationFrame(animate);
            // Smooth input once per frame so both stars and glow shapes share the same eased motion.
            smoothInput();
            if (timestamp - lastFrame < frameInterval) return;
            lastFrame = timestamp;

            ctx.clearRect(0, 0, width, height);
            stars.forEach(drawStar);
            spawnShootingStar();
            shootingStars.forEach((s, i) => {
                s.x += s.vx;
                s.y += s.vy;
                s.life -= s.decay;
                drawShootingStar(s);
                if (s.life <= 0 || s.x > width + 100 || s.y > height + 100) {
                    shootingStars.splice(i, 1);
                }
            });
        }

        resize();
        window.addEventListener('resize', resize);
        requestAnimationFrame(animate);
    }
})();
