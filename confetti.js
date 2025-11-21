/**
 * Confetti Animation Module
 * Creates celebratory confetti effect for successful trades
 */

function createConfetti() {
    const container = document.body;
    const colors = [
        '#2ad6ff', '#7fffd4', '#00d966', '#ffb700', '#ff4444',
        '#00ffff', '#ff00ff', '#00ff00', '#ff0000', '#0000ff',
        '#ffff00', '#ff6600', '#ff0099', '#00ffff', '#00ff99'
    ];

    for (let i = 0; i < 100; i++) {
        const confetti = document.createElement('div');
        confetti.style.position = 'fixed';
        confetti.style.pointerEvents = 'none';
        confetti.style.left = window.innerWidth / 2 + 'px';
        confetti.style.top = window.innerHeight / 2 + 'px';

        // Random shape: circles, squares, or diamonds
        const shape = Math.random();
        const randomColor = colors[Math.floor(Math.random() * colors.length)];

        if (shape < 0.33) {
            // Circle
            confetti.style.width = '15px';
            confetti.style.height = '15px';
            confetti.style.backgroundColor = randomColor;
            confetti.style.borderRadius = '50%';
        } else if (shape < 0.66) {
            // Square
            confetti.style.width = '12px';
            confetti.style.height = '12px';
            confetti.style.backgroundColor = randomColor;
        } else {
            // Diamond/Triangle
            confetti.style.width = '0';
            confetti.style.height = '0';
            confetti.style.borderLeft = '8px solid transparent';
            confetti.style.borderRight = '8px solid transparent';
            confetti.style.borderTop = `15px solid ${randomColor}`;
        }

        container.appendChild(confetti);

        // Calculate explosion trajectory
        const angle = Math.random() * Math.PI * 2;
        const velocity = Math.random() * 400 + 250;
        const vx = Math.cos(angle) * velocity;
        const vy = Math.sin(angle) * velocity;
        const duration = Math.random() * 2.5 + 2;

        // Animate horizontal movement (constant velocity)
        gsap.to(confetti, {
            x: vx,
            opacity: 0,
            rotation: Math.random() * 1440 + 720,
            duration: duration,
            ease: "none",
            onComplete: () => confetti.remove()
        });

        // Animate vertical movement with gravity effect
        gsap.to(confetti, {
            y: vy * 2 + 500, // Goes up first (vy * 2) then gravity pulls it down (+500)
            duration: duration,
            ease: "sine.in"
        });
    }
}
