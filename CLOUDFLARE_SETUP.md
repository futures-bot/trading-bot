# Cloudflare Worker Setup for Trading Bot API Proxy

This guide sets up a free Cloudflare Worker to proxy your bot's analytics API, enabling:
- CORS headers for browser dashboards
- Geo-routing (request routing from anywhere globally)
- DDoS protection and caching
- **100K requests/day free** (more than enough for the bot)

## Prerequisites

1. **Cloudflare Account** (free tier): https://dash.cloudflare.com/
2. **Node.js 18+** and npm installed locally
3. **Your GCP VM's external IP**: `35.196.16.129` (or your current VM IP)

## Step 1: Create Cloudflare Project Locally

```bash
npm install -g wrangler

# Create a new project (answer "No" to git, "No" to TypeScript)
wrangler init trading-bot-api
cd trading-bot-api
```

## Step 2: Create the Worker Code

Create `src/index.js`:

```javascript
export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    
    // Proxy /api/* requests to your bot
    if (url.pathname.startsWith('/api/')) {
      // Replace with your GCP VM's external IP or domain
      const botUrl = 'http://35.196.16.129:8080' + url.pathname + url.search;
      
      try {
        const response = await fetch(botUrl, {
          method: request.method,
          headers: request.headers,
          timeout: 30000,
        });
        
        // Add CORS headers
        const newHeaders = new Headers(response.headers);
        newHeaders.set('Access-Control-Allow-Origin', '*');
        newHeaders.set('Access-Control-Allow-Methods', 'GET, OPTIONS, HEAD');
        newHeaders.set('Access-Control-Allow-Headers', 'Content-Type, Authorization');
        newHeaders.set('Access-Control-Max-Age', '3600');
        
        return new Response(response.body, {
          status: response.status,
          headers: newHeaders,
        });
      } catch (err) {
        return new Response(JSON.stringify({
          error: 'Failed to reach bot API',
          details: err.message
        }), {
          status: 503,
          headers: {
            'Content-Type': 'application/json',
            'Access-Control-Allow-Origin': '*',
          }
        });
      }
    }
    
    // Handle OPTIONS requests (CORS preflight)
    if (request.method === 'OPTIONS') {
      return new Response(null, {
        headers: {
          'Access-Control-Allow-Origin': '*',
          'Access-Control-Allow-Methods': 'GET, OPTIONS, HEAD',
          'Access-Control-Allow-Headers': 'Content-Type, Authorization',
          'Access-Control-Max-Age': '3600',
        }
      });
    }
    
    // Not found
    return new Response('Not found. Use /api/* endpoints.', { status: 404 });
  }
};
```

## Step 3: Configure wrangler.toml

Edit `wrangler.toml`:

```toml
name = "trading-bot-api"
main = "src/index.js"
compatibility_date = "2024-01-15"
compatibility_flags = []

[env.production]
name = "trading-bot-api"
route = "api.yourdomain.com/*"
zone_id = "YOUR_ZONE_ID"  # You'll get this from Cloudflare
```

You can skip the `route` and `zone_id` if you don't have a custom domain yet.

## Step 4: Authenticate with Cloudflare

```bash
wrangler login
# This opens a browser to authorize your account
# After authorization, you'll see your account info
```

## Step 5: Deploy the Worker

```bash
# Deploy to Cloudflare
wrangler deploy

# Output:
# ✓ Uploaded trading-bot-api
# ✓ Deployed to https://trading-bot-api.YOUR_USERNAME.workers.dev
```

## Step 6: Test the Worker

Replace `YOUR_USERNAME` with your actual username:

```bash
# Test health check
curl https://trading-bot-api.YOUR_USERNAME.workers.dev/api/health

# Test stats
curl https://trading-bot-api.YOUR_USERNAME.workers.dev/api/stats

# Test latest sessions
curl https://trading-bot-api.YOUR_USERNAME.workers.dev/api/sessions/latest
```

## Step 7: Use in Dashboard

Now you can call the API from a browser dashboard without CORS issues:

```javascript
const API_BASE = 'https://trading-bot-api.YOUR_USERNAME.workers.dev';

async function getStats() {
  const res = await fetch(`${API_BASE}/api/stats`);
  const data = await res.json();
  return data;
}

// Use this in your dashboard
setInterval(getStats, 5000);
```

## Optional: Custom Domain

If you want to use your own domain:

1. Add domain to Cloudflare: https://dash.cloudflare.com/
2. Update `wrangler.toml` with your zone ID (found in domain settings)
3. Redeploy: `wrangler deploy`

Example with custom domain:

```toml
route = "api.yourdomain.com/*"
zone_id = "YOUR_ZONE_ID"
```

Then deploy again and use `https://api.yourdomain.com/api/stats`

## Monitoring Usage

View worker analytics and requests:

```bash
# View worker stats
wrangler tail

# Or go to: https://dash.cloudflare.com/YOUR_ACCOUNT/workers/view
```

You'll see:
- Total requests per day
- Response times
- Error rates
- Cache hit rate

## Updating Bot IP

If your GCP VM IP changes, update the worker:

```javascript
// In src/index.js, change:
const botUrl = 'http://NEW_IP:8080' + url.pathname + url.search;

// Then redeploy:
wrangler deploy
```

## Troubleshooting

### Worker returns 503 "Failed to reach bot API"

1. Check your GCP VM is running: `make vm-status`
2. Check API is listening: `gcloud compute ssh trading-bot --zone=us-east1-c --command='curl http://localhost:8080/api/health'`
3. Verify firewall allows outbound HTTPS from Cloudflare

### CORS errors in browser dashboard

Check that your Worker has these headers:
```javascript
'Access-Control-Allow-Origin': '*'
'Access-Control-Allow-Methods': 'GET, OPTIONS, HEAD'
```

### High latency

Worker processes requests in ~50-200ms. If slow:
1. Check GCP VM CPU: `make vm-status`
2. Check network latency: `make api-health` (should be <100ms)
3. Enable caching in Worker for frequently-accessed endpoints

## Rate Limits

**Cloudflare Workers Free Tier:**
- 100,000 requests/day
- No charge beyond this (requests simply fail with 429)
- No credit card required

**Bot API usage estimate:**
- Scraper: 24 requests/day
- Paper/Testnet: 9,000 requests/day (every 10 seconds)
- Dashboard polling: Variable (1 req every 5-60 seconds depending on refresh rate)
- **Total: ~9,000-15,000 requests/day**

You're using **~10% of the free limit**, so upgrades are never needed.

## Next Steps

1. Create a dashboard HTML file that calls the Worker
2. Host the dashboard on GitHub Pages, Vercel, or another static host
3. Access your dashboard from anywhere in the world via CORS-enabled API

Example dashboard template is in the API.md file.

