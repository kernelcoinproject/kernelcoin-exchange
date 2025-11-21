class SlideCaptcha {
    constructor(containerId) {
        this.container = document.getElementById(containerId);
        this.captchaData = null;
        this.isCompleted = false;
        this.isDragging = false;
        this.startX = 0;
        this.startY = 0;
        this.currentX = 0;
        this.currentY = 0;
        this.tile = null;
        this.puzzle = null;
        
        this.init();
    }

    async init() {
        try {
            await this.loadCaptcha();
            this.render();
            this.bindEvents();
        } catch (error) {
            console.error('Failed to initialize captcha:', error);
            this.showError('Failed to load captcha');
        }
    }

    async loadCaptcha() {
        const response = await fetch('/api/captcha/generate');
        if (!response.ok) {
            throw new Error('Failed to generate captcha');
        }
        this.captchaData = await response.json();
        this.isCompleted = false;
        // Reset register button when loading new captcha
        this.updateRegisterButton(false);
    }

    render() {
        this.container.innerHTML = `
            <div class="captcha-puzzle" id="puzzle-${this.container.id}">
                <button class="captcha-refresh" onclick="window.captchaInstances['${this.container.id}'].refresh()" title="Refresh captcha">↻</button>
                <div class="captcha-tile" id="tile-${this.container.id}"></div>
            </div>
            <div class="captcha-instructions">Drag the puzzle piece to fill the gap</div>
        `;

        this.puzzle = document.getElementById(`puzzle-${this.container.id}`);
        this.tile = document.getElementById(`tile-${this.container.id}`);

        // Set background images and check their dimensions
        this.puzzle.style.backgroundImage = `url(${this.captchaData.master_image})`;
        this.tile.style.backgroundImage = `url(${this.captchaData.tile_image})`;
        
        // Check actual image dimensions
        this.checkImageDimensions();
        
        // Store target coordinates for validation
        this.serverTargetX = this.captchaData.tile_x;
        this.serverTargetY = this.captchaData.tile_y;
        
        // Position tile at starting position (left side, same Y as target)
        const startX = 0;
        const startY = this.captchaData.tile_y;
        this.tile.style.left = `${startX}px`;
        this.tile.style.top = `${startY}px`;
        
        this.currentX = startX;
        this.currentY = startY;
        
        // Initialize register button state
        this.updateRegisterButton(false);
    }

    updateRegisterButton(isCompleted) {
        const registerBtn = document.querySelector('button[type="submit"]');
        if (registerBtn && registerBtn.closest('#registerForm')) {
            if (isCompleted) {
                registerBtn.style.backgroundColor = '#4CAF50';
                registerBtn.style.color = 'white';
                registerBtn.disabled = false;
            } else {
                registerBtn.style.backgroundColor = '#cccccc';
                registerBtn.style.color = '#666666';
                registerBtn.disabled = true;
            }
        }
    }

    bindEvents() {
        // Only bind mousedown and touchstart initially
        this.tile.addEventListener('mousedown', this.onMouseDown.bind(this));
        this.tile.addEventListener('touchstart', this.onTouchStart.bind(this));
    }

    onMouseDown(e) {
        if (this.isCompleted) return;
        
        e.preventDefault();
        e.stopPropagation();
        
        const rect = this.puzzle.getBoundingClientRect();
        this.isDragging = true;
        this.startX = e.clientX - rect.left - this.currentX;
        this.startY = e.clientY - rect.top - this.currentY;
        this.tile.classList.add('dragging');
        
        document.body.style.userSelect = 'none';
        
        // Add event listeners dynamically like the official library
        document.addEventListener('mousemove', this.onMouseMove.bind(this), false);
        document.addEventListener('mouseup', this.onMouseUp.bind(this), false);
        document.addEventListener('mouseleave', this.onMouseUp.bind(this), false);
    }

    onMouseMove(e) {
        if (!this.isDragging || this.isCompleted) return;
        
        e.preventDefault();
        e.stopPropagation();
        
        const rect = this.puzzle.getBoundingClientRect();
        const newX = e.clientX - rect.left - this.startX;
        const newY = e.clientY - rect.top - this.startY;
        
        this.updatePosition(newX, newY);
    }

    onMouseUp(e) {
        if (!this.isDragging) return;
        
        e.preventDefault();
        e.stopPropagation();
        
        this.clearMouseEvents();
        this.isDragging = false;
        this.tile.classList.remove('dragging');
        document.body.style.userSelect = '';
        
        this.checkCompletion();
    }
    
    clearMouseEvents() {
        document.removeEventListener('mousemove', this.onMouseMove);
        document.removeEventListener('mouseup', this.onMouseUp);
        document.removeEventListener('mouseleave', this.onMouseUp);
    }

    onTouchStart(e) {
        if (this.isCompleted) return;
        
        e.preventDefault();
        e.stopPropagation();
        
        const touch = e.touches[0];
        const rect = this.puzzle.getBoundingClientRect();
        this.isDragging = true;
        this.startX = touch.clientX - rect.left - this.currentX;
        this.startY = touch.clientY - rect.top - this.currentY;
        this.tile.classList.add('dragging');
        
        // Add touch event listeners dynamically
        document.addEventListener('touchmove', this.onTouchMove.bind(this), { passive: false });
        document.addEventListener('touchend', this.onTouchEnd.bind(this), false);
    }

    onTouchMove(e) {
        if (!this.isDragging || this.isCompleted) return;
        
        e.preventDefault();
        e.stopPropagation();
        
        const touch = e.touches[0];
        const rect = this.puzzle.getBoundingClientRect();
        const newX = touch.clientX - rect.left - this.startX;
        const newY = touch.clientY - rect.top - this.startY;
        
        this.updatePosition(newX, newY);
    }

    onTouchEnd(e) {
        if (!this.isDragging) return;
        
        e.preventDefault();
        e.stopPropagation();
        
        this.clearTouchEvents();
        this.isDragging = false;
        this.tile.classList.remove('dragging');
        this.checkCompletion();
    }
    
    clearTouchEvents() {
        document.removeEventListener('touchmove', this.onTouchMove);
        document.removeEventListener('touchend', this.onTouchEnd);
    }

    updatePosition(x, y) {
        // Constrain to full puzzle boundaries using actual tile dimensions
        const tileWidth = this.tileWidth || 60; // Default to 60 if not set yet
        const tileHeight = this.tileHeight || 60; // Default to 60 if not set yet
        const maxX = 300 - tileWidth;
        const maxY = 220 - tileHeight;
        
        x = Math.max(0, Math.min(x, maxX));
        y = Math.max(0, Math.min(y, maxY));
        
        this.currentX = x;
        this.currentY = y;
        
        this.tile.style.left = `${x}px`;
        this.tile.style.top = `${y}px`;
        

    }

    checkCompletion() {
        const tolerance = 15;
        const deltaX = Math.abs(this.currentX - this.serverTargetX);
        const deltaY = Math.abs(this.currentY - this.serverTargetY);
        
        if (deltaX <= tolerance && deltaY <= tolerance) {
            this.showSuccess();
            this.isCompleted = true;
            this.updateRegisterButton(true);
        } else {
            this.showBriefError();
        }
    }

    showSuccess() {
        // Remove any existing success message
        const existingSuccess = this.puzzle.querySelector('.captcha-success');
        if (existingSuccess) {
            existingSuccess.remove();
        }
        
        const successDiv = document.createElement('div');
        successDiv.className = 'captcha-success';
        successDiv.innerHTML = 'Confirmed';
        successDiv.style.cssText = 'position: absolute; top: 0; left: 0; right: 0; height: 40px; background: rgba(76, 175, 80, 0.9); display: flex; align-items: center; justify-content: center; color: white; font-weight: bold; font-size: 16px; z-index: 30;';
        this.puzzle.appendChild(successDiv);
        
        // Store reference to remove on form submission
        this.successDiv = successDiv;
        
        // Hide the "Are you human?" label
        const captchaLabel = document.getElementById('regCaptchaLabel');
        if (captchaLabel) {
            captchaLabel.style.display = 'none';
        }
        
        // Hide entire captcha after 1 second
        setTimeout(() => {
            this.container.style.display = 'none';
        }, 1000);
    }

    showBriefError() {
        // Add a subtle shake animation or brief red border
        this.tile.style.border = '2px solid #f44336';
        setTimeout(() => {
            this.tile.style.border = '';
        }, 500);
    }

    showError(message) {
        this.container.innerHTML = `
            <div class="captcha-error" style="position: relative; height: 100px;">
                ${message}
                <button onclick="window.captchaInstances['${this.container.id}'].refresh()" 
                        style="display: block; margin: 10px auto; padding: 5px 10px;">
                    Try Again
                </button>
            </div>
        `;
    }

    async refresh() {
        this.container.innerHTML = '<div class="captcha-loading">Loading captcha...</div>';
        try {
            await this.loadCaptcha();
            this.render();
            this.bindEvents();
        } catch (error) {
            console.error('Failed to refresh captcha:', error);
            this.showError('Failed to load captcha');
        }
    }

    getCaptchaData() {
        if (!this.isCompleted) {
            return null;
        }
        
        // Clear success message when form is submitted
        this.clearSuccess();
        
        return {
            captcha_id: this.captchaData.captcha_id,
            captcha_x: Math.round(this.currentX),
            captcha_y: Math.round(this.currentY)
        };
    }

    clearSuccess() {
        if (this.successDiv && this.successDiv.parentNode) {
            this.successDiv.remove();
            this.successDiv = null;
        }
    }

    checkImageDimensions() {
        // Check master image dimensions
        const masterImg = new Image();
        masterImg.onload = () => {
        };
        masterImg.src = this.captchaData.master_image;
        
        // Check tile image dimensions and adjust tile size
        const tileImg = new Image();
        tileImg.onload = () => {
            // Update tile to match actual image size
            this.tile.style.width = `${tileImg.width}px`;
            this.tile.style.height = `${tileImg.height}px`;
            
            // Store tile dimensions for boundary calculations
            this.tileWidth = tileImg.width;
            this.tileHeight = tileImg.height;
        };
        tileImg.src = this.captchaData.tile_image;
    }

    isValid() {
        return this.isCompleted;
    }
}

// Global captcha instances
window.captchaInstances = {};

// Initialize captcha for auth forms - only for registration
function initAuthCaptchas() {
    if (document.getElementById('regCaptchaContainer') && !window.captchaInstances['regCaptchaContainer']) {
        window.captchaInstances['regCaptchaContainer'] = new SlideCaptcha('regCaptchaContainer');
    }
    
    // Remove login captcha initialization
    // if (document.getElementById('loginCaptchaContainer') && !window.captchaInstances['loginCaptchaContainer']) {
    //     window.captchaInstances['loginCaptchaContainer'] = new SlideCaptcha('loginCaptchaContainer');
    // }
}

// Make function globally available
window.initAuthCaptchas = initAuthCaptchas;