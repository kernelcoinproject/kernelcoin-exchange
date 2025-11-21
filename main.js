let currentUser = null;
let currentUserId = null;
let tradesTable = null;
let adminTable = null;
let myTradesTable = null;
let buyOrdersTable = null;
let sellOrdersTable = null;
let buyOrdersDataTable = null;
let sellOrdersDataTable = null;

// Tab switching
document.addEventListener('click', (e) => {
    if (e.target.classList.contains('tab-switch')) {
        const tabName = e.target.dataset.tab;
        
        // Only allow access to non-exchange tabs when logged in
        if (!currentUser && tabName !== 'exchange') {
            return;
        }

        // Admin panel only for admin user
        if (tabName === 'admin' && currentUser !== 'admin') {
            alert('Admin access denied');
            return;
        }

        showTab(tabName);
    }
});

function showTab(tabName) {
    // Save current tab to localStorage
    localStorage.setItem('currentTab', tabName);
    
    // Load tab if not already loaded
    loadTabOnDemand(tabName).then(() => {
        document.querySelectorAll('.tab-content').forEach(tab => {
            tab.classList.remove('active');
        });
        const tabElement = document.getElementById(tabName);
        if (tabElement) {
            tabElement.classList.add('active');
        }

        // Setup form listeners after tab is loaded
        if (tabName === 'auth') {
            setupFormEventListeners();
            if (window.initAuthCaptchas) {
                window.initAuthCaptchas();
            }
        }
        if (tabName === 'exchange') {
            loadTrades();
            setupExchangeEventListeners();
        }
        if (tabName === 'trades') {
            loadMyTrades();
            setupMyTradesEventListeners();
        }
        if (tabName === 'balances') {
            loadMyBalances();
            updateLtcPriceDisplay();
            updateKcnPriceStats();
            setupBalancesEventListeners();
        }
        if (tabName === 'wallet') {
            loadWalletData();
            setupWalletEventListeners();
        }
        if (tabName === 'admin') {
            if (ltcUsdPrice === 0) {
                updateLtcPriceDisplay().then(() => loadAdminData());
            } else {
                loadAdminData();
            }
        }
    });
}

// Setup form event listeners after tabs are loaded
function setupFormEventListeners() {
    // Registration
    const registerForm = document.getElementById('registerForm');
    if (registerForm && !registerForm.hasAttribute('data-listener-added')) {
        registerForm.setAttribute('data-listener-added', 'true');
        // Add real-time validation
        const regUsername = document.getElementById('regUsername');
        
        if (regUsername) {
            regUsername.addEventListener('input', (e) => {
                const value = e.target.value;
                const isValid = /^[a-zA-Z0-9]*$/.test(value) && value.length <= 64;
                e.target.style.borderColor = isValid ? '' : 'var(--error)';
                if (!isValid && value.length > 0) {
                    e.target.title = 'Only letters and numbers allowed, max 64 characters';
                } else {
                    e.target.title = '';
                }
            });
        }
        
        const regPassword = document.getElementById('regPassword');
        const regConfirmPassword = document.getElementById('regConfirmPassword');
        
        [regPassword, regConfirmPassword].forEach(input => {
            if (input) {
                input.addEventListener('input', (e) => {
                    const value = e.target.value;
                    const isValid = value.length <= 64;
                    e.target.style.borderColor = isValid ? '' : 'var(--error)';
                    if (!isValid) {
                        e.target.title = 'Password must be less than 64 characters';
                    } else {
                        e.target.title = '';
                    }
                    
                    // Check if passwords match and show captcha
                    const password = regPassword.value;
                    const confirmPassword = regConfirmPassword.value;
                    const captchaGroup = document.getElementById('regCaptchaGroup');
                    
                    if (password.length > 0 && confirmPassword.length > 0 && password === confirmPassword) {
                        if (captchaGroup && captchaGroup.style.display === 'none') {
                            captchaGroup.style.display = 'block';
                            // Initialize captcha when first shown
                            if (window.initAuthCaptchas) {
                                window.initAuthCaptchas();
                            }
                        }
                    } else {
                        if (captchaGroup) {
                            captchaGroup.style.display = 'none';
                        }
                    }
                });
            }
        });
        
        registerForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const username = document.getElementById('regUsername').value;
            const password = document.getElementById('regPassword').value;
            const confirmPassword = document.getElementById('regConfirmPassword').value;

            if (password !== confirmPassword) {
                alert('Passwords do not match');
                return;
            }

            // Get captcha data
            const captchaInstance = window.captchaInstances['regCaptchaContainer'];
            if (!captchaInstance || !captchaInstance.isValid()) {
                alert('Please complete the captcha');
                return;
            }
            
            const captchaData = captchaInstance.getCaptchaData();
            
            try {
                const response = await fetch('/api/register', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include',
                    body: JSON.stringify({ 
                        username, 
                        password,
                        captcha_id: captchaData.captcha_id,
                        captcha_x: captchaData.captcha_x,
                        captcha_y: captchaData.captcha_y
                    })
                });
                const data = await response.json();

                const messageDiv = document.getElementById('registerMessage');
                if (data.success) {
                    messageDiv.style.display = 'block';
                    messageDiv.style.backgroundColor = '#d4edda';
                    messageDiv.style.color = '#155724';
                    messageDiv.style.border = '1px solid #c3e6cb';
                    messageDiv.textContent = 'Account created! You can now login.';
                    document.getElementById('registerForm').reset();
                } else {
                    messageDiv.style.display = 'block';
                    messageDiv.style.backgroundColor = '#f8d7da';
                    messageDiv.style.color = '#721c24';
                    messageDiv.style.border = '1px solid #f5c6cb';
                    messageDiv.textContent = data.error;
                }
                
                // Always refresh captcha after form submission
                if (captchaInstance && captchaInstance.refresh) {
                    captchaInstance.refresh();
                }
            } catch (error) {
                alert('Error: ' + error.message);
            }
        });
    }

    // Login
    const loginForm = document.getElementById('loginForm');
    if (loginForm) {
        loginForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const username = document.getElementById('loginUsername').value;
            const password = document.getElementById('loginPassword').value;
            
            try {
                const response = await fetch('/api/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include',
                    body: JSON.stringify({ 
                        username, 
                        password
                    })
                });
                const data = await response.json();

                if (data.success) {
                    currentUser = data.username;
                    currentUserId = data.user_id;
                    document.getElementById('currentUser').textContent = data.username;
                    const logoutBtn = document.getElementById('logoutBtn');
                    if (logoutBtn) {
                        logoutBtn.classList.remove('hidden');
                        logoutBtn.style.display = 'inline-block';
                    }
                    if (data.username === 'admin') {
                        const adminBtn = document.getElementById('adminTabBtn');
                        if (adminBtn) {
                            adminBtn.style.display = 'inline-block';
                        }
                    }
                    const authTab = document.getElementById('auth');
                    if (authTab) {
                        authTab.classList.remove('active');
                    }
                    showTab('exchange');
                } else {
                    alert('Error: ' + data.error);
                }
            } catch (error) {
                alert('Error: ' + error.message);
            }
        });
    }
}

async function loadTrades() {
    try {
        const response = await fetch('/api/trade/list', { credentials: 'include' });
        const data = await response.json();
        let buyOrders = [];
        let sellOrders = [];
        if (data && data.trades && Array.isArray(data.trades)) {
            data.trades.forEach(trade => {
                if (trade.coin_buying === 'kernelcoin') buyOrders.push(trade);
                if (trade.coin_selling === 'kernelcoin') sellOrders.push(trade);
            });
        }
        buyOrders.sort((a, b) => (a.price_per_unit * ltcUsdPrice) - (b.price_per_unit * ltcUsdPrice));
        sellOrders.sort((a, b) => (a.price_per_unit * ltcUsdPrice) - (b.price_per_unit * ltcUsdPrice));

        // Clean up existing DataTables
        if ($.fn.DataTable.isDataTable('#buyOrdersTable')) {
            $('#buyOrdersTable').DataTable().destroy();
            $('#buyOrdersTable_wrapper').remove();
        }
        const buyTbody = document.querySelector('#buyOrdersTable tbody');
        if (buyTbody) {
            buyTbody.innerHTML = '';
            if (buyOrders.length > 0) {
                buyOrders.forEach(trade => {
                    const row = buyTbody.insertRow();
                    const isOwnTrade = currentUser && Number(trade.seller_id) === Number(currentUserId);
                    const kcnAmount = trade.coin_buying === 'kernelcoin' ? trade.amount_buying : trade.amount_selling;
                    const ltcAmount = trade.coin_selling === 'litecoin' ? trade.amount_selling : trade.amount_buying;
                    const totalPriceUsd = (ltcAmount * ltcUsdPrice).toFixed(8);
                    const pricePerCoin = (trade.price_per_unit * ltcUsdPrice).toFixed(8);
                    let actionBtn = '';
                    if (isOwnTrade) {
                        actionBtn = `<button class="btn btn-danger btn-small" onclick="executeTradeModal(${trade.id}, ${kcnAmount}, ${trade.price_per_unit}, '${trade.seller_name}', 'sell', true)">Cancel</button>`;
                    } else {
                        actionBtn = `<button class="btn btn-primary btn-small" onclick="executeTradeModal(${trade.id}, ${kcnAmount}, ${trade.price_per_unit}, '${trade.seller_name}', 'sell', false)">Execute</button>`;
                    }
                    row.innerHTML = `<td>${trade.seller_name}</td><td>${kcnAmount.toFixed(8)}</td><td>${ltcAmount.toFixed(8)}</td><td>$${totalPriceUsd}</td><td>$${pricePerCoin}</td><td>${actionBtn}</td>`;
                });
                if (!buyOrdersDataTable) {
                    buyOrdersDataTable = new DataTable('#buyOrdersTable', { pageLength: 10, responsive: true, destroy: true });
                }
            } else {
                buyTbody.innerHTML = '<tr><td colspan="6" class="text-center">No buy orders available</td></tr>';
            }
        }

        if ($.fn.DataTable.isDataTable('#sellOrdersTable')) {
            $('#sellOrdersTable').DataTable().destroy();
            $('#sellOrdersTable_wrapper').remove();
        }
        const sellTbody = document.querySelector('#sellOrdersTable tbody');
        if (sellTbody) {
            sellTbody.innerHTML = '';
            if (sellOrders.length > 0) {
                sellOrders.forEach(trade => {
                    const row = sellTbody.insertRow();
                    const isOwnTrade = currentUser && Number(trade.seller_id) === Number(currentUserId);
                    const kcnAmount = trade.coin_selling === 'kernelcoin' ? trade.amount_selling : trade.amount_buying;
                    const ltcAmount = trade.coin_buying === 'litecoin' ? trade.amount_buying : trade.amount_selling;
                    const totalPriceUsd = (ltcAmount * ltcUsdPrice).toFixed(8);
                    const pricePerCoin = (trade.price_per_unit * ltcUsdPrice).toFixed(8);
                    let actionBtn = '';
                    if (isOwnTrade) {
                        actionBtn = `<button class="btn btn-danger btn-small" onclick="executeTradeModal(${trade.id}, ${kcnAmount}, ${trade.price_per_unit}, '${trade.seller_name}', 'buy', true)">Cancel</button>`;
                    } else {
                        actionBtn = `<button class="btn btn-primary btn-small" onclick="executeTradeModal(${trade.id}, ${kcnAmount}, ${trade.price_per_unit}, '${trade.seller_name}', 'buy', false)">Execute</button>`;
                    }
                    row.innerHTML = `<td>${trade.seller_name}</td><td>${kcnAmount.toFixed(8)}</td><td>${ltcAmount.toFixed(8)}</td><td>$${totalPriceUsd}</td><td>$${pricePerCoin}</td><td>${actionBtn}</td>`;
                });
                if (!sellOrdersDataTable) {
                    sellOrdersDataTable = new DataTable('#sellOrdersTable', { pageLength: 10, responsive: true, destroy: true });
                }
            } else {
                sellTbody.innerHTML = '<tr><td colspan="6" class="text-center">No sell orders available</td></tr>';
            }
        }
    } catch (error) {
        console.error('Error loading trades:', error);
    }
}

let currentBalances = { litecoin: 0, kernelcoin: 0, ltc_reserved: 0, kcn_reserved: 0 };

async function loadMyBalances() {
    if (!currentUser) return;

    try {
        const response = await fetch('/api/balance', {
            credentials: 'include'
        });
        const data = await response.json();

        if (data.error) {
            console.error('Balance error:', data.error);
            return;
        }

        // Store balances in global variable
        currentBalances = {
            litecoin: data.litecoin || 0,
            kernelcoin: data.kernelcoin || 0,
            ltc_reserved: data.ltc_reserved || 0,
            kcn_reserved: data.kcn_reserved || 0
        };

        // Update DOM elements if they exist
        const myLtcBalance = document.getElementById('myLtcBalance');
        const myKcnBalance = document.getElementById('myKcnBalance');
        const myLtcReserved = document.getElementById('myLtcReserved');
        const myKcnReserved = document.getElementById('myKcnReserved');
        
        if (myLtcBalance) myLtcBalance.textContent = currentBalances.litecoin.toFixed(8);
        if (myKcnBalance) myKcnBalance.textContent = currentBalances.kernelcoin.toFixed(8);
        if (myLtcReserved) myLtcReserved.textContent = currentBalances.ltc_reserved.toFixed(8);
        if (myKcnReserved) myKcnReserved.textContent = currentBalances.kcn_reserved.toFixed(8);
        
        // Update balance displays on my-trades tab
        const myLtcBalance2 = document.getElementById('myLtcBalance2');
        const myKcnBalance2 = document.getElementById('myKcnBalance2');
        if (myLtcBalance2) {
            myLtcBalance2.textContent = currentBalances.litecoin.toFixed(8);
        }
        if (myKcnBalance2) {
            myKcnBalance2.textContent = currentBalances.kernelcoin.toFixed(8);
        }
    } catch (error) {
        console.error('Error loading balances:', error);
    }
}

async function loadAdminData() {
    if (currentUser !== 'admin') {
        alert('Admin access denied');
        return;
    }

    try {
        const [escrowResponse, tradesResponse, adminResponse, statsResponse] = await Promise.all([
            fetch('/api/escrow', { credentials: 'include' }),
            fetch('/api/trade/list', { credentials: 'include' }),
            fetch('/api/admin', { credentials: 'include' }),
            fetch('/api/admin/stats', { credentials: 'include' })
        ]);
        
        const escrowData = await escrowResponse.json();
        const tradesData = await tradesResponse.json();
        const adminData = await adminResponse.json();
        const statsData = await statsResponse.json();

        // Calculate total reserved amounts
        let totalLtcReserved = 0;
        let totalKcnReserved = 0;
        if (adminData && adminData.users) {
            adminData.users.forEach(user => {
                totalLtcReserved += user.ltc_reserved || 0;
                totalKcnReserved += user.kcn_reserved || 0;
            });
        }

        // Update stats
        document.getElementById('totalUsers').textContent = escrowData.total_users || 0;
        document.getElementById('totalLtcValue').textContent = (escrowData.total_litecoin || 0).toFixed(8);
        document.getElementById('totalKcnValue').textContent = (escrowData.total_kernelcoin || 0).toFixed(8);
        document.getElementById('totalLtcReserved').textContent = totalLtcReserved.toFixed(8);
        document.getElementById('totalKcnReserved').textContent = totalKcnReserved.toFixed(8);
        
        // Update LTC price in admin tab
        const adminLtcPrice = document.getElementById('ltcPrice');
        if (adminLtcPrice) {
            adminLtcPrice.textContent = '$' + ltcUsdPrice.toFixed(2);
        }
        
        // Count active and completed trades
        let activeTrades = 0;
        if (tradesData && tradesData.trades) {
            activeTrades = tradesData.trades.filter(t => t.status === 'open').length;
        }
        
        document.getElementById('activeTrades').textContent = activeTrades;
        document.getElementById('completedTrades').textContent = statsData.completed_trades || 0;

        if (adminTable) {
            adminTable.destroy();
        }

        const tbody = document.querySelector('#escrowTable tbody');
        tbody.innerHTML = '';

        if (adminData && adminData.users && Array.isArray(adminData.users) && adminData.users.length > 0) {
            adminData.users.forEach(user => {
                const row = tbody.insertRow();
                const usdValue = (user.litecoin_balance * ltcUsdPrice).toFixed(2);
                row.innerHTML = `
                    <td>${user.username}</td>
                    <td>${user.litecoin_balance.toFixed(8)}</td>
                    <td>${user.kernelcoin_balance.toFixed(8)}</td>
                    <td>${(user.ltc_reserved || 0).toFixed(8)}</td>
                    <td>${(user.kcn_reserved || 0).toFixed(8)}</td>
                    <td>$${usdValue}</td>
                `;
            });

            adminTable = new DataTable('#escrowTable', {
                pageLength: 20,
                responsive: true,
                destroy: true
            });
        } else {
            tbody.innerHTML = '<tr><td colspan="6" class="text-center">No users found</td></tr>';
        }
    } catch (error) {
        console.error('Error loading admin data:', error);
        alert('Error loading admin data');
    }
}

async function loadMyTrades() {
    if (!currentUserId) return;
    try {
        const response = await fetch('/api/trade/my-trades', {
            credentials: 'include'
        });
        const data = await response.json();
        
        if ($.fn.DataTable.isDataTable('#myTradesTable')) {
            $('#myTradesTable').DataTable().destroy();
            $('#myTradesTable_wrapper').remove();
        }

        const tbody = document.querySelector('#myTradesTable tbody');
        tbody.innerHTML = '';

        if (data && data.trades && Array.isArray(data.trades) && data.trades.length > 0) {
            data.trades.forEach(trade => {
                const row = tbody.insertRow();

                let statusBadge;
                if (trade.status === 'open') {
                    statusBadge = `<span class="status-badge status-open">Open</span>`;
                } else if (trade.status === 'cancelled') {
                    statusBadge = `<span class="status-badge status-cancelled">Cancelled</span>`;
                } else {
                    statusBadge = `<span class="status-badge status-completed">Closed</span>`;
                }

                const actionBtn = trade.status === 'open' ?
                    `<button class="btn btn-danger" onclick="cancelTrade(${trade.id})" style="font-size: 0.85rem; padding: 0.5rem 1rem;">Cancel</button>` :
                    '-';

                const tradeType = trade.coin_selling === 'kernelcoin' ? 'Sell' : 'Buy';
                const kcnAmount = trade.coin_selling === 'kernelcoin' ? trade.amount_selling : trade.amount_buying;
                const ltcAmount = trade.coin_selling === 'litecoin' ? trade.amount_selling : trade.amount_buying;
                const priceUsd = (ltcAmount * ltcUsdPrice).toFixed(8);
                const actualPricePerCoin = ltcAmount / kcnAmount;
                const pricePerCoinUsd = (actualPricePerCoin * ltcUsdPrice).toFixed(8);
                const dateCreated = new Date(trade.created_at).toLocaleString();
                const counterparty = trade.counterparty || 'You';
                
                row.innerHTML = `
                    <td>${tradeType}</td>
                    <td>${counterparty}</td>
                    <td>${kcnAmount.toFixed(8)}</td>
                    <td>${ltcAmount.toFixed(8)}</td>
                    <td>$${priceUsd}</td>
                    <td>$${pricePerCoinUsd}</td>
                    <td>${dateCreated}</td>
                    <td>-</td>
                    <td>${statusBadge}</td>
                    <td>${actionBtn}</td>
                `;
            });
            if (!myTradesTable) {
                myTradesTable = new DataTable('#myTradesTable', {
                    pageLength: 10,
                    responsive: true,
                    destroy: true
                });
            }
        } else {
            tbody.innerHTML = '<tr><td colspan="10" class="text-center">No trades yet</td></tr>';
        }
    } catch (error) {
        console.error('Error loading trades:', error);
    }
}

let ltcUsdPrice = 0;

// Update LTC price display periodically
function updateLtcPriceDisplay() {
    return fetch('/api/ltc-price', {
        credentials: 'include'
    })
        .then(response => response.json())
        .then(data => {
            if (data.ltc_usd) {
                ltcUsdPrice = data.ltc_usd;
                const ltcPriceElement = document.getElementById('ltcPrice');
                const ltcPriceInBalanceElement = document.getElementById('ltcPriceInBalance');
                if (ltcPriceElement) {
                    ltcPriceElement.textContent = '$' + ltcUsdPrice.toFixed(2);
                }
                if (ltcPriceInBalanceElement) {
                    ltcPriceInBalanceElement.textContent = ltcUsdPrice.toFixed(2);
                }
            }
        })
        .catch(e => console.error('Error fetching LTC price:', e));
}

// Check session on page load
async function checkSessionOnLoad() {
    try {
        const response = await fetch('/api/session', {
            credentials: 'include'
        });
        const data = await response.json();

        if (data.success) {
            currentUser = data.username;
            currentUserId = data.user_id;
            document.getElementById('currentUser').textContent = data.username;
            const logoutBtn = document.getElementById('logoutBtn');
            if (logoutBtn) {
                logoutBtn.classList.remove('hidden');
                logoutBtn.style.display = 'inline-block';
            }
            if (data.username === 'admin') {
                const adminBtn = document.getElementById('adminTabBtn');
                if (adminBtn) {
                    adminBtn.style.display = 'inline-block';
                }
            }
            
            // Restore last active tab or default to exchange
            const lastTab = localStorage.getItem('currentTab');
            const validTabs = ['exchange', 'trades', 'balances', 'wallet', 'admin'];
            const tabToShow = (lastTab && validTabs.includes(lastTab)) ? lastTab : 'exchange';
            
            // Check admin access
            if (tabToShow === 'admin' && data.username !== 'admin') {
                showTab('exchange');
            } else {
                showTab(tabToShow);
            }
        } else {
            showTab('auth');
        }
    } catch (error) {
        console.error('Error checking session:', error);
    }
}

// Check session on page load
checkSessionOnLoad();

// Setup initial form listeners after a short delay to ensure tabs are loaded
setTimeout(() => {
    setupFormEventListeners();
}, 100);

// Initial fetch and periodic updates
updateLtcPriceDisplay();
setInterval(() => {
    updateLtcPriceDisplay();
    loadTrades();
}, 30000); // Update every 30 seconds

// Setup exchange event listeners
function setupExchangeEventListeners() {
    const createTradeForm = document.getElementById('createTradeForm');
    if (createTradeForm && !createTradeForm.hasAttribute('data-listener-added')) {
        createTradeForm.setAttribute('data-listener-added', 'true');
        createTradeForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            console.log('Form submitted');

            const kcnAmount = parseFloat(document.getElementById('kcnAmount').value);
            const ltcAmount = parseFloat(document.getElementById('ltcAmount').value);
            console.log('Amounts:', kcnAmount, ltcAmount);

            if (kcnAmount <= 0 || ltcAmount <= 0) {
                alert('Please enter valid amounts');
                return;
            }

            // Show confirmation modal
            const sellingCoin = document.getElementById('sellingCoin').value;
            const tradeType = sellingCoin === 'litecoin' ? 'BUY' : 'SELL';
            const estimatedCost = (ltcAmount * ltcUsdPrice).toFixed(2);
            console.log('Opening modal with:', tradeType, estimatedCost);
            
            document.getElementById('createTradeMessage').textContent = 
                `Are you sure you want to create a ${tradeType} listing?`;
            document.getElementById('modalKcnAmount').textContent = kcnAmount.toFixed(8);
            document.getElementById('modalLtcAmount').textContent = ltcAmount.toFixed(8);
            document.getElementById('modalEstimatedCost').textContent = `$${estimatedCost}`;
            
            document.getElementById('createTradeModal').classList.add('active');
        });
    }

    const tradePriceInput = document.getElementById('tradePrice');
    if (tradePriceInput && !tradePriceInput.hasAttribute('data-listener-added')) {
        tradePriceInput.setAttribute('data-listener-added', 'true');
        tradePriceInput.addEventListener('input', () => {
            const price = parseFloat(document.getElementById('tradePrice').value) || 0;
            const usdPrice = price * ltcUsdPrice;
            const usdPriceElement = document.getElementById('tradePriceUsd');
            if (usdPriceElement) {
                usdPriceElement.value = usdPrice.toFixed(8);
            }

            const amount = parseFloat(document.getElementById('tradeAmount').value) || 0;
            const totalLtc = amount * price;
            const totalLtcElement = document.getElementById('totalLtcDisplay');
            if (totalLtcElement) {
                totalLtcElement.textContent = totalLtc.toFixed(8);
            }
        });
    }

    const tradeAmountInput = document.getElementById('tradeAmount');
    if (tradeAmountInput && !tradeAmountInput.hasAttribute('data-listener-added')) {
        tradeAmountInput.setAttribute('data-listener-added', 'true');
        tradeAmountInput.addEventListener('input', () => {
            const amount = parseFloat(document.getElementById('tradeAmount').value) || 0;
            const price = parseFloat(document.getElementById('tradePrice').value) || 0;
            const totalLtc = amount * price;
            const totalLtcElement = document.getElementById('totalLtcDisplay');
            if (totalLtcElement) {
                totalLtcElement.textContent = totalLtc.toFixed(8);
            }
        });
    }
}

// Setup my-trades event listeners
function setupMyTradesEventListeners() {
    const createTradeForm = document.getElementById('createTradeForm');
    if (createTradeForm && !createTradeForm.hasAttribute('data-listener-added')) {
        createTradeForm.setAttribute('data-listener-added', 'true');
        createTradeForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const tradeType = document.getElementById('tradeType').value;
            const kcnAmount = parseFloat(document.getElementById('tradeAmount').value);
            const ltcAmount = parseFloat(document.getElementById('tradeLtcAmount').value);

            try {
                const response = await fetch('/api/trade/create', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include',
                    body: JSON.stringify({
                        coin_selling: tradeType === 'buy' ? 'litecoin' : 'kernelcoin',
                        amount_selling: tradeType === 'buy' ? ltcAmount : kcnAmount,
                        coin_buying: tradeType === 'buy' ? 'kernelcoin' : 'litecoin',
                        amount_buying: tradeType === 'buy' ? kcnAmount : ltcAmount
                    })
                });
                const data = await response.json();

                if (data.success) {
                    alert('Listing created!');
                    createTradeForm.reset();
                    loadMyTrades();
                    loadMyBalances();
                } else {
                    alert('Error: ' + data.error);
                }
            } catch (error) {
                alert('Error: ' + error.message);
            }
        });
    }

    // No additional input listeners needed for direct amount inputs
}

// Clear wallet data on logout
function clearWalletData() {
    const elements = [
        'walletLtcBalance', 'walletKcnBalance', 'walletLtcReserved', 'walletKcnReserved',
        'ltcAddress', 'kcnAddress', 'ltcReceiveAddress', 'kcnReceiveAddress',
        'newLtcAddress', 'newKcnAddress'
    ];
    
    elements.forEach(id => {
        const element = document.getElementById(id);
        if (element) {
            if (element.tagName === 'INPUT') {
                element.value = '';
            } else {
                element.textContent = '';
            }
        }
    });
    
    // Clear transaction table
    const tbody = document.getElementById('transactionTableBody');
    if (tbody) {
        tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; color: #888;">No transactions found</td></tr>';
    }
}

// Load wallet data
async function loadWalletData() {
    if (!currentUser) return;
    
    try {
        // Load balance
        const balanceResponse = await fetch('/api/balance', { credentials: 'include' });
        const balanceData = await balanceResponse.json();
        
        if (!balanceData.error) {
            document.getElementById('walletLtcBalance').textContent = (balanceData.litecoin || 0).toFixed(8);
            document.getElementById('walletKcnBalance').textContent = (balanceData.kernelcoin || 0).toFixed(8);
            document.getElementById('walletLtcReserved').textContent = (balanceData.ltc_reserved || 0).toFixed(8);
            document.getElementById('walletKcnReserved').textContent = (balanceData.kcn_reserved || 0).toFixed(8);
        }
        
        // Load user addresses from database
        const userResponse = await fetch('/api/user', { credentials: 'include' });
        const userData = await userResponse.json();
        
        if (!userData.error) {
            document.getElementById('ltcAddress').textContent = userData.litecoin_address || '';
            document.getElementById('kcnAddress').textContent = userData.kernelcoin_address || '';
            document.getElementById('ltcReceiveAddress').textContent = (userData.litecoin_receive_address && userData.litecoin_receive_address.trim()) ? userData.litecoin_receive_address : 'Not generated';
            document.getElementById('kcnReceiveAddress').textContent = (userData.kernelcoin_receive_address && userData.kernelcoin_receive_address.trim()) ? userData.kernelcoin_receive_address : 'Not generated';
            
            // Pre-fill address update form
            const newLtcAddr = document.getElementById('newLtcAddress');
            const newKcnAddr = document.getElementById('newKcnAddress');
            if (newLtcAddr) newLtcAddr.value = userData.litecoin_address || '';
            if (newKcnAddr) newKcnAddr.value = userData.kernelcoin_address || '';
        }
        
        // Load withdrawal fee
        const feeResponse = await fetch('/api/withdraw-fee', { credentials: 'include' });
        const feeData = await feeResponse.json();
        
        if (!feeData.error) {
            const ltcFee = feeData.ltc_withdraw_fee || 0;
            const usdFee = feeData.ltc_withdraw_fee_usd || 0;
            const feeElement = document.getElementById('ltcWithdrawFee');
            if (feeElement) {
                feeElement.textContent = `${ltcFee.toFixed(8)} LTC (~$${usdFee.toFixed(4)})`;
            }
        }
        
        // Load transaction history
        loadTransactionHistory();
    } catch (error) {
        console.error('Error loading wallet data:', error);
    }
}

// Load transaction history
async function loadTransactionHistory() {
    try {
        const response = await fetch('/api/transactions', { credentials: 'include' });
        const data = await response.json();
        
        const tbody = document.getElementById('transactionTableBody');
        if (!tbody) return;
        
        tbody.innerHTML = '';
        
        if (data.transactions && data.transactions.length > 0) {
            data.transactions.forEach(tx => {
                const row = tbody.insertRow();
                const statusClass = tx.status === 'confirmed' ? 'status-completed' : 'status-open';
                const txHashDisplay = tx.tx_hash ? 
                    `<span style="font-family: monospace; font-size: 0.8em; cursor: pointer; color: var(--primary); text-decoration: underline;" onclick="showTxHashModal('${tx.tx_hash}')">${tx.tx_hash.substring(0, 16)}...</span>` : 
                    '-';
                
                row.innerHTML = `
                    <td>${new Date(tx.created_at).toLocaleString()}</td>
                    <td><span class="status-badge ${tx.type === 'deposit' ? 'status-completed' : 'status-cancelled'}">${tx.type.toUpperCase()}</span></td>
                    <td>${tx.coin.toUpperCase()}</td>
                    <td>${tx.amount.toFixed(8)}</td>
                    <td><span class="status-badge ${statusClass}">${tx.status.toUpperCase()}</span></td>
                    <td>${txHashDisplay}</td>
                `;
            });
        } else {
            tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; color: #888;">No transactions found</td></tr>';
        }
    } catch (error) {
        console.error('Error loading transaction history:', error);
    }
}

// Show transaction hash modal
function showTxHashModal(txHash) {
    const modal = document.createElement('div');
    modal.style.cssText = 'position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.8); display: flex; align-items: center; justify-content: center; z-index: 10000;';
    
    const content = document.createElement('div');
    content.style.cssText = 'background: var(--bg-card); padding: 2rem; border-radius: 12px; border: 2px solid var(--border-color); max-width: 700px; width: 95%;';
    
    content.innerHTML = `
        <h3 style="color: var(--primary); margin-bottom: 1rem;">Transaction Hash</h3>
        <div style="background: rgba(20, 20, 20, 0.8); padding: 1rem; border-radius: 6px; font-family: monospace; word-break: break-all; margin-bottom: 1rem; border: 1px solid var(--border-color);">${txHash}</div>
        <button onclick="this.closest('div').parentElement.remove()" style="background: var(--primary); color: #000; border: none; padding: 0.5rem 1rem; border-radius: 6px; cursor: pointer;">Close</button>
    `;
    
    modal.appendChild(content);
    document.body.appendChild(modal);
    
    modal.addEventListener('click', (e) => {
        if (e.target === modal) {
            document.body.removeChild(modal);
        }
    });
}

// Check confirmations function
async function checkConfirmations(coin) {
    try {
        const response = await fetch('/api/check-confirmations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ coin: coin })
        });
        const data = await response.json();

        if (data.success) {
            if (data.message) {
                alert(data.message);
            } else {
                alert(`Confirmations found! 50 ${coin} added to balance.`);
            }
            loadWalletData();
        } else {
            if (data.message && data.status === 'pending') {
                alert(data.message);
            } else {
                alert('Error: ' + data.error);
            }
        }
    } catch (error) {
        alert('Error: ' + error.message);
    }
}

// Generate receive address function
async function generateReceiveAddress(coin) {
    try {
        const response = await fetch('/api/generate-receive-address', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ coin: coin })
        });
        const data = await response.json();

        if (data.success) {
            alert(`${coin.charAt(0).toUpperCase() + coin.slice(1)} receive address generated!`);
            loadWalletData();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (error) {
        alert('Error: ' + error.message);
    }
}

// Update single address function
async function updateSingleAddress(coin) {
    const inputId = coin === 'litecoin' ? 'newLtcAddress' : 'newKcnAddress';
    const address = document.getElementById(inputId).value;
    
    if (!address.trim()) {
        alert('Please enter an address');
        return;
    }
    
    // Validate address format
    if (!/^[a-zA-Z0-9]*$/.test(address) || address.length > 64) {
        alert('Invalid address format. Only letters and numbers allowed, max 64 characters.');
        return;
    }
    
    try {
        const body = {};
        body[coin + '_address'] = address;
        
        const response = await fetch('/api/update-addresses', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify(body)
        });
        const data = await response.json();

        if (data.success) {
            alert(`${coin.charAt(0).toUpperCase() + coin.slice(1)} address updated successfully!`);
            loadWalletData();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (error) {
        alert('Error: ' + error.message);
    }
}

// Setup wallet event listeners
function setupWalletEventListeners() {
    // Add real-time validation for address inputs
    const newLtcAddr = document.getElementById('newLtcAddress');
    const newKcnAddr = document.getElementById('newKcnAddress');
    
    [newLtcAddr, newKcnAddr].forEach(input => {
        if (input && !input.hasAttribute('data-listener-added')) {
            input.setAttribute('data-listener-added', 'true');
            input.addEventListener('input', (e) => {
                const value = e.target.value;
                const isValid = /^[a-zA-Z0-9]*$/.test(value) && value.length <= 64;
                e.target.style.borderColor = isValid ? '' : 'var(--error)';
                if (!isValid && value.length > 0) {
                    e.target.title = 'Only letters and numbers allowed, max 64 characters';
                } else {
                    e.target.title = '';
                }
            });
        }
    });

    
    const changePasswordForm = document.getElementById('changePasswordForm');
    if (changePasswordForm && !changePasswordForm.hasAttribute('data-listener-added')) {
        changePasswordForm.setAttribute('data-listener-added', 'true');
        changePasswordForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const currentPassword = document.getElementById('currentPassword').value;
            const newPassword = document.getElementById('newPassword').value;
            const confirmPassword = document.getElementById('confirmPassword').value;

            if (newPassword !== confirmPassword) {
                alert('New passwords do not match');
                return;
            }

            try {
                const response = await fetch('/api/change-password', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include',
                    body: JSON.stringify({
                        current_password: currentPassword,
                        new_password: newPassword
                    })
                });
                const data = await response.json();

                if (data.success) {
                    alert('Password changed successfully!');
                    changePasswordForm.reset();
                } else {
                    alert('Error: ' + data.error);
                }
            } catch (error) {
                alert('Error: ' + error.message);
            }
        });
    }
}

let withdrawCoin = null;

// Request withdrawal function
function requestWithdraw(coin) {
    withdrawCoin = coin;
    document.getElementById('withdrawModalTitle').textContent = `Withdraw ${coin.toUpperCase()}`;
    document.getElementById('withdrawAmountLabel').textContent = `Amount (${coin.toUpperCase()})`;
    document.getElementById('withdrawAmount').value = '';
    document.getElementById('withdrawModal').classList.add('active');
}

function closeWithdrawModal() {
    document.getElementById('withdrawModal').classList.remove('active');
    withdrawCoin = null;
}

function updateEstimatedCost() {
    const ltcAmount = parseFloat(document.getElementById('ltcAmount').value) || 0;
    const kcnAmount = parseFloat(document.getElementById('kcnAmount').value) || 0;
    const estimatedCost = ltcAmount * ltcUsdPrice;
    const estimatedCostElement = document.getElementById('estimatedCostUsd');
    const pricePerCoinElement = document.getElementById('pricePerCoinUsd');
    const priceBreakdownElement = document.getElementById('priceBreakdown');
    
    if (estimatedCostElement) {
        estimatedCostElement.value = '$' + estimatedCost.toFixed(2);
    }
    
    if (pricePerCoinElement && kcnAmount > 0) {
        const pricePerCoin = (ltcAmount / kcnAmount) * ltcUsdPrice;
        pricePerCoinElement.value = '$' + pricePerCoin.toFixed(8);
    } else if (pricePerCoinElement) {
        pricePerCoinElement.value = '$0.00';
    }
    
    const summaryElement = document.querySelector('#transactionSummary > div:nth-child(2)');
    if (summaryElement && kcnAmount > 0 && ltcAmount > 0) {
        const pricePerCoin = (ltcAmount / kcnAmount) * ltcUsdPrice;
        const tradeType = document.getElementById('sellingCoin').value === 'litecoin' ? 'BUY' : 'SELL';
        summaryElement.textContent = `${tradeType} ${kcnAmount.toFixed(8)} KCN with ${ltcAmount.toFixed(8)} LTC at $${pricePerCoin.toFixed(2)} per coin for a total of $${estimatedCost.toFixed(2)}`;
    }
}

window.closeCreateTradeModal = function() {
    document.getElementById('createTradeModal').classList.remove('active');
}

window.openTestModal = function() {
    document.getElementById('createTradeMessage').textContent = 'Test modal opening';
    document.getElementById('modalKcnAmount').textContent = '1.00000000';
    document.getElementById('modalLtcAmount').textContent = '0.10000000';
    document.getElementById('modalEstimatedCost').textContent = '$100.00';
    document.getElementById('createTradeModal').classList.add('active');
}

window.showCreateModal = function() {
    const kcnAmount = parseFloat(document.getElementById('kcnAmount').value);
    const ltcAmount = parseFloat(document.getElementById('ltcAmount').value);

    if (kcnAmount <= 0 || ltcAmount <= 0) {
        alert('Please enter valid amounts');
        return;
    }

    const sellingCoin = document.getElementById('sellingCoin').value;
    const tradeType = sellingCoin === 'litecoin' ? 'buy' : 'sell';
    const pricePerCoin = (ltcAmount / kcnAmount * ltcUsdPrice).toFixed(8);
    const totalCost = (ltcAmount * ltcUsdPrice).toFixed(2);
    
    document.getElementById('createTradeMessage').textContent = 
        `Are you sure you want to create an order to ${tradeType} ${kcnAmount.toFixed(8)} KCN for ${ltcAmount.toFixed(8)} LTC at $${pricePerCoin} per coin for a total of $${totalCost}? These funds will be locked until the orders complete or are canceled.`;
    
    document.getElementById('createTradeModal').classList.add('active');
}

window.confirmCreateTrade = async function() {
    const sellingCoin = document.getElementById('sellingCoin').value;
    const kcnAmount = parseFloat(document.getElementById('kcnAmount').value);
    const ltcAmount = parseFloat(document.getElementById('ltcAmount').value);

    try {
        const response = await fetch('/api/trade/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({
                coin_selling: sellingCoin,
                amount_selling: sellingCoin === 'litecoin' ? ltcAmount : kcnAmount,
                coin_buying: sellingCoin === 'litecoin' ? 'kernelcoin' : 'litecoin',
                amount_buying: sellingCoin === 'litecoin' ? kcnAmount : ltcAmount
            })
        });
        const data = await response.json();

        closeCreateTradeModal();
        
        if (data.success) {
            createConfetti();
            showSuccessModal('Listing created!');
            document.getElementById('createTradeForm').reset();
            updateEstimatedCost();
            loadMyBalances();
            setTimeout(() => loadTrades(), 1000);
        } else {
            alert('Error: ' + data.error);
        }
    } catch (error) {
        closeCreateTradeModal();
        alert('Error: ' + error.message);
    }
}

function confirmWithdraw() {
    const amount = parseFloat(document.getElementById('withdrawAmount').value);
    if (!amount || amount <= 0) {
        alert('Please enter a valid amount');
        return;
    }
    
    fetch('/api/withdraw', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ coin: withdrawCoin, amount: amount })
    })
    .then(response => response.json())
    .then(data => {
        closeWithdrawModal();
        if (data.success) {
            alert('Withdrawal request submitted!');
            loadWalletData();
        } else {
            alert('Error: ' + data.error);
        }
    })
    .catch(error => {
        closeWithdrawModal();
        alert('Error: ' + error.message);
    });
}

// Handler functions for buy/sell buttons
function buyKcnHandler() {
    setupTradeForm('buy');
}

function sellKcnHandler() {
    setupTradeForm('sell');
}

// Setup balances event listeners
function setupBalancesEventListeners() {
    const kcnAmountInput = document.getElementById('kcnAmount');
    const ltcAmountInput = document.getElementById('ltcAmount');
    
    if (kcnAmountInput && !kcnAmountInput.hasAttribute('data-listener-added')) {
        kcnAmountInput.setAttribute('data-listener-added', 'true');
        kcnAmountInput.addEventListener('input', () => {
            const kcnAmount = parseFloat(document.getElementById('kcnAmount').value) || 0;
            const ltcAmount = parseFloat(document.getElementById('ltcAmount').value) || 0;
            if (kcnAmount > 0 && ltcAmount > 0) {
                updateEstimatedCost();
            }
        });
    }
    
    if (ltcAmountInput && !ltcAmountInput.hasAttribute('data-listener-added')) {
        ltcAmountInput.setAttribute('data-listener-added', 'true');
        ltcAmountInput.addEventListener('input', updateEstimatedCost);
    }

    const buyKcnBtn = document.getElementById('buyKcnBtn');
    const sellKcnBtn = document.getElementById('sellKcnBtn');
    
    if (buyKcnBtn) {
        buyKcnBtn.removeEventListener('click', buyKcnHandler);
        buyKcnBtn.addEventListener('click', buyKcnHandler);
    }
    
    if (sellKcnBtn) {
        sellKcnBtn.removeEventListener('click', sellKcnHandler);
        sellKcnBtn.addEventListener('click', sellKcnHandler);
    }
    
    // Modal button listeners
    const confirmBtn = document.getElementById('confirmCreateTradeBtn');
    const cancelBtn = document.getElementById('cancelCreateTradeBtn');
    
    if (confirmBtn && !confirmBtn.hasAttribute('data-listener-added')) {
        confirmBtn.setAttribute('data-listener-added', 'true');
        confirmBtn.addEventListener('click', confirmCreateTrade);
    }
    
    if (cancelBtn && !cancelBtn.hasAttribute('data-listener-added')) {
        cancelBtn.setAttribute('data-listener-added', 'true');
        cancelBtn.addEventListener('click', closeCreateTradeModal);
    }
}

function setupTradeForm(mode) {
    const form = document.getElementById('createTradeForm');
    if (!form) return;
    
    form.style.display = 'block';
    
    if (mode === 'buy') {
        document.getElementById('sellingCoin').value = 'litecoin';
        document.getElementById('buyingCoin').value = 'kernelcoin';
        updateButtonStyles('buy');
    } else {
        document.getElementById('sellingCoin').value = 'kernelcoin';
        document.getElementById('buyingCoin').value = 'litecoin';
        updateButtonStyles('sell');
    }
    
    updateTransactionSummary(mode);
}

function updateButtonStyles(mode) {
    const buyBtn = document.getElementById('buyKcnBtn');
    const sellBtn = document.getElementById('sellKcnBtn');
    
    if (buyBtn && sellBtn) {
        if (mode === 'buy') {
            buyBtn.style.opacity = '1';
            buyBtn.style.transform = 'scale(1.05)';
            sellBtn.style.opacity = '0.6';
            sellBtn.style.transform = 'scale(1)';
        } else {
            sellBtn.style.opacity = '1';
            sellBtn.style.transform = 'scale(1.05)';
            buyBtn.style.opacity = '0.6';
            buyBtn.style.transform = 'scale(1)';
        }
    }
}

function updateTransactionSummary(mode) {
    const summaryElement = document.querySelector('#transactionSummary > div:nth-child(2)');
    if (summaryElement) {
        if (mode === 'buy') {
            summaryElement.textContent = 'BUY KCN with LTC';
        } else {
            summaryElement.textContent = 'SELL KCN for LTC';
        }
    }
}

// Update KCN price statistics
async function updateKcnPriceStats() {
    try {
        const response = await fetch('/api/trade/list', { credentials: 'include' });
        const data = await response.json();

        let kcnBuyTotal = 0, kcnBuyCount = 0, kcnBuyMin = Infinity;
        let kcnSellTotal = 0, kcnSellCount = 0, kcnSellMin = Infinity;

        if (data && data.trades && Array.isArray(data.trades)) {
            data.trades.forEach(trade => {
                if (trade.coin_buying === 'kernelcoin') {
                    kcnBuyTotal += trade.price_per_unit;
                    kcnBuyCount++;
                    kcnBuyMin = Math.min(kcnBuyMin, trade.price_per_unit);
                }
                if (trade.coin_selling === 'kernelcoin') {
                    kcnSellTotal += trade.price_per_unit;
                    kcnSellCount++;
                    kcnSellMin = Math.min(kcnSellMin, trade.price_per_unit);
                }
            });
        }

        const avgKcnBuyPrice = kcnBuyCount > 0 ? kcnBuyTotal / kcnBuyCount : 0;
        const avgKcnSellPrice = kcnSellCount > 0 ? kcnSellTotal / kcnSellCount : 0;
        const lowestKcnBuyPrice = kcnBuyMin === Infinity ? 0 : kcnBuyMin;
        const lowestKcnSellPrice = kcnSellMin === Infinity ? 0 : kcnSellMin;

        // Update display elements if they exist
        const elements = {
            'lowestKcnBuyPrice': (lowestKcnBuyPrice * ltcUsdPrice).toFixed(8),
            'lowestKcnBuyPriceLtc': lowestKcnBuyPrice.toFixed(8),
            'avgKcnBuyPrice': (avgKcnBuyPrice * ltcUsdPrice).toFixed(8),
            'avgKcnBuyPriceLtc': avgKcnBuyPrice.toFixed(8),
            'lowestKcnSellPrice': (lowestKcnSellPrice * ltcUsdPrice).toFixed(8),
            'lowestKcnSellPriceLtc': lowestKcnSellPrice.toFixed(8),
            'avgKcnSellPrice': (avgKcnSellPrice * ltcUsdPrice).toFixed(8),
            'avgKcnSellPriceLtc': avgKcnSellPrice.toFixed(8)
        };
        
        Object.keys(elements).forEach(id => {
            const element = document.getElementById(id);
            if (element) {
                element.textContent = elements[id];
            }
        });
    } catch (e) {
        console.error('Error calculating KCN price stats:', e);
    }
}

let pendingExecution = null;

async function executeTradeModal(tradeId, amount, pricePerUnit, sellerName, orderType, isOwnOrder) {
    if (!currentUser && !isOwnOrder) {
        alert('Please log in to execute trades');
        return;
    }
    
    // Load current balances first
    await loadMyBalances();

    const totalLtc = amount * pricePerUnit;
    const totalUsd = totalLtc * ltcUsdPrice;
    const pricePerCoin = pricePerUnit * ltcUsdPrice;

    let message = '';
    if (isOwnOrder) {
        message = 'Are you sure you want to remove this listing?';
    } else if (orderType === 'sell') {
        message = `Are you sure you want to sell ${amount.toFixed(8)} KCN to ${sellerName} for ${totalLtc.toFixed(8)} LTC ($${totalUsd.toFixed(8)}) at $${pricePerCoin.toFixed(8)} per coin?`;
    } else {
        message = `Are you sure you want to buy ${amount.toFixed(8)} KCN from ${sellerName} for ${totalLtc.toFixed(8)} LTC ($${totalUsd.toFixed(8)}) at $${pricePerCoin.toFixed(8)} per coin?`;
    }

    const modalMessage = document.getElementById('modalMessage');
    const modalAmount = document.getElementById('modalAmount');
    const modalPrice = document.getElementById('modalPrice');
    const modalTotalUsd = document.getElementById('modalTotalUsd');
    const modalPricePerCoin = document.getElementById('modalPricePerCoin');
    
    if (modalMessage) modalMessage.textContent = message;

    const warningElement = document.querySelector('.modal-warning');
    if (isOwnOrder) {
        warningElement.style.display = 'none';
    } else {
        warningElement.style.display = 'block';
    }

    const confirmBtn = document.getElementById('confirmBtn');
    const cancelBtn = document.getElementById('cancelBtn');

    if (isOwnOrder) {
        confirmBtn.style.display = 'block';
        confirmBtn.textContent = 'Remove Listing';
        confirmBtn.style.background = 'linear-gradient(135deg, #ff4444 0%, #cc0000 100%)';
        confirmBtn.style.color = '#fff';
        cancelBtn.textContent = 'Keep Listing';
        cancelBtn.style.background = 'linear-gradient(135deg, #00d966 0%, #00a84d 100%)';
        cancelBtn.style.color = '#000';
    } else {
        const myLtcBalance = currentBalances.litecoin;
        const myKcnBalance = currentBalances.kernelcoin;

        const insufficientBalance = orderType !== 'sell' && totalLtc > myLtcBalance;
        const insufficientKcnBalance = orderType === 'sell' && amount > myKcnBalance;

        if (insufficientBalance || insufficientKcnBalance) {
            confirmBtn.style.display = 'none';
            cancelBtn.textContent = 'Close';
            cancelBtn.className = 'btn-cancel';
            const insufficientMsg = orderType === 'sell' ?
                `You don't have enough Kernelcoin. You have ${myKcnBalance.toFixed(8)} KCN.` :
                `You don't have enough Litecoin. You have ${myLtcBalance.toFixed(8)} LTC.`;
            if (modalMessage) modalMessage.textContent = insufficientMsg;
            const tradeDetail = document.querySelector('.trade-detail');
            const modalWarning = document.querySelector('.modal-warning');
            if (tradeDetail) tradeDetail.style.display = 'none';
            if (modalWarning) modalWarning.style.display = 'none';
        } else {
            confirmBtn.style.display = 'block';
            confirmBtn.textContent = 'Confirm';
            confirmBtn.className = 'btn-confirm';
            cancelBtn.textContent = 'Cancel';
            cancelBtn.className = 'btn-cancel';
            const tradeDetail = document.querySelector('.trade-detail');
            const modalWarning = document.querySelector('.modal-warning');
            if (tradeDetail) tradeDetail.style.display = 'block';
            if (modalWarning) modalWarning.style.display = 'block';
        }
    }

    pendingExecution = {
        tradeId: tradeId,
        amount: amount,
        isOwnOrder: isOwnOrder
    };

    document.getElementById('executionModal').classList.add('active');
}

function cancelExecution() {
    document.getElementById('executionModal').classList.remove('active');
    pendingExecution = null;
}

async function confirmExecution() {
    if (!pendingExecution) return;

    if (pendingExecution.isOwnOrder) {
        await performCancelTrade(pendingExecution.tradeId);
    } else {
        try {
            const response = await fetch('/api/trade/execute', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'include',
                body: JSON.stringify({
                    trade_id: pendingExecution.tradeId,
                    quantity: pendingExecution.amount
                })
            });
            const data = await response.json();

            document.getElementById('executionModal').classList.remove('active');
            pendingExecution = null;

            if (data.success) {
                createConfetti();
                showSuccessModal('Order executed!');
                loadTrades();
                loadMyBalances();
            } else {
                await loadMyBalances();
                const myLtcBalance = parseFloat(document.getElementById('myLtcBalance').innerText) || 0;
                const myKcnBalance = parseFloat(document.getElementById('myKcnBalance').innerText) || 0;

                if (data.error && data.error.includes('Insufficient balance')) {
                    alert(`Trade failed: Insufficient balance.\n\nYour current balances:\nLTC: ${myLtcBalance.toFixed(8)}\nKCN: ${myKcnBalance.toFixed(8)}`);
                } else {
                    alert('Trade failed: ' + data.error);
                }
            }
        } catch (error) {
            alert('Error: ' + error.message);
        }
    }
}

async function performCancelTrade(tradeId) {
    try {
        const response = await fetch('/api/trade/cancel', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ trade_id: tradeId })
        });
        const data = await response.json();

        document.getElementById('executionModal').classList.remove('active');
        pendingExecution = null;

        if (data.success) {
            alert('Listing cancelled!');
            loadMyTrades();
            loadMyBalances();
            loadTrades();
        } else {
            alert('Cancel failed: ' + data.error);
        }
    } catch (error) {
        alert('Error: ' + error.message);
    }
}

async function cancelTrade(tradeId) {
    if (!confirm('Cancel this listing?')) return;

    try {
        const response = await fetch('/api/trade/cancel', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ trade_id: tradeId })
        });
        const data = await response.json();

        if (data.success) {
            alert('Listing cancelled!');
            loadMyTrades();
            loadMyBalances();
            loadTrades();
        } else {
            alert('Cancel failed: ' + data.error);
        }
    } catch (error) {
        alert('Error: ' + error.message);
    }
}

// Setup logout button listener
function setupLogoutButton() {
    const logoutBtn = document.getElementById('logoutBtn');
    if (logoutBtn && !logoutBtn.hasAttribute('data-logout-listener')) {
        logoutBtn.setAttribute('data-logout-listener', 'true');
        logoutBtn.addEventListener('click', async () => {
            try {
                await fetch('/api/logout', {
                    method: 'POST',
                    credentials: 'include'
                });
                currentUser = null;
                currentUserId = null;
                document.getElementById('currentUser').textContent = 'Not logged in';
                logoutBtn.style.display = 'none';
                const adminBtn = document.getElementById('adminTabBtn');
                if (adminBtn) {
                    adminBtn.style.display = 'none';
                }
                clearWalletData();
                document.querySelectorAll('.tab-content').forEach(tab => tab.classList.remove('active'));
                showTab('auth');
                const loginForm = document.getElementById('loginForm');
                const registerForm = document.getElementById('registerForm');
                if (loginForm) loginForm.reset();
                if (registerForm) registerForm.reset();
            } catch (error) {
                console.error('Logout error:', error);
            }
        });
    }
}

// Setup logout button on page load
setupLogoutButton();

function showSuccessModal(message) {
    const modal = document.createElement('div');
    modal.style.cssText = 'position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 10000;';
    
    const content = document.createElement('div');
    content.style.cssText = 'background: var(--bg-card); padding: 2rem; border-radius: 8px; text-align: center; color: #00d966; font-size: 1.5rem; font-weight: bold; border: 2px solid #00d966;';
    content.textContent = message;
    
    modal.appendChild(content);
    document.body.appendChild(modal);
    
    setTimeout(() => {
        document.body.removeChild(modal);
    }, 2000);
}

function showErrorModal(message) {
    const modal = document.createElement('div');
    modal.style.cssText = 'position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 10000;';
    
    const content = document.createElement('div');
    content.style.cssText = 'background: var(--bg-card); padding: 2rem; border-radius: 8px; text-align: center; color: #ff4444; font-size: 1.5rem; font-weight: bold; border: 2px solid #ff4444;';
    content.textContent = message;
    
    modal.appendChild(content);
    document.body.appendChild(modal);
    
    setTimeout(() => {
        document.body.removeChild(modal);
    }, 2000);
}