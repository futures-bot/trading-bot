# Deploy Bot API Proxy to Vercel

Deploy the trading bot analytics proxy on Vercel with your custom domain in minutes.

## Prerequisites

1. **Vercel Account**: https://vercel.com (free tier)
2. **Custom Domain**: Already registered and available
3. **GCP Bot IP**: `35.196.16.129` (or your current VM IP)

## Option 1: Simple Node.js Express Proxy (Recommended)

### Step 1: Create project structure

```bash
mkdir trading-bot-proxy
cd trading-bot-proxy
npm init -y
npm install express cors
```

### Step 2: Create `index.js`

```javascript
const express = require('express');
const cors = require('cors');
const http = require('http');

const app = express();
app.use(cors());

const BOT_IP = '35.196.16.129';
const BOT_PORT = 8080;

app.get('/api/*', async (req, res) => {
  const path = req.path;
  const query = req.url.split('?')[1] || '';
  const url = `http://${BOT_IP}:${BOT_PORT}${path}${query ? '?' + query : ''}`;

  try {
    const response = await fetch(url, {
      method: 'GET',
      timeout: 10000,
    });

    const data = await response.text();
    
    res.set({
      'Content-Type': 'application/json',
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'GET, OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type',
    });
    
    res.status(response.status).send(data);
  } catch (err) {
    res.status(503).json({
      error: 'Failed to reach bot API',
      details: err.message
    });
  }
});

app.options('*', cors());

app.get('/', (req, res) => {
  res.json({
    message: 'Trading Bot Analytics Proxy',
    endpoints: [
      '/api/health',
      '/api/stats',
      '/api/trades',
      '/api/sessions',
      '/api/sessions/latest',
      '/api/performance'
    ]
  });
});

const PORT = process.env.PORT || 3000;
app.listen(PORT, () => {
  console.log(`Proxy running on port ${PORT}`);
});
```

### Step 3: Update `package.json`

Add this to scripts:

```json
{
  "name": "trading-bot-proxy",
  "version": "1.0.0",
  "scripts": {
    "start": "node index.js",
    "dev": "node index.js"
  },
  "dependencies": {
    "express": "^4.18.2",
    "cors": "^2.8.5"
  }
}
```

### Step 4: Create `vercel.json`

```json
{
  "version": 2,
  "builds": [
    {
      "src": "index.js",
      "use": "@vercel/node"
    }
  ],
  "routes": [
    {
      "src": "/(.*)",
      "dest": "index.js"
    }
  ],
  "env": {
    "BOT_IP": "35.196.16.129"
  }
}
```

### Step 5: Initialize git and deploy

```bash
git init
git add .
git commit -m "Initial proxy setup"

vercel --prod
```

Follow the prompts:
- Link to existing project? **No**
- Project name: `trading-bot-proxy`
- Deploy from current directory: **Yes**

### Step 6: Link custom domain

In Vercel dashboard:
1. Go to your project → Settings → Domains
2. Add custom domain (e.g., `api.yourdomain.com`)
3. Vercel shows you DNS records to add
4. Add them to your domain registrar (GoDaddy, Namecheap, etc.)
5. Wait 5-10 minutes for DNS propagation

## Option 2: Vercel Serverless Functions (No external dependencies)

### Step 1: Create project structure

```bash
mkdir trading-bot-proxy
cd trading-bot-proxy
npm init -y
```

### Step 2: Create `api/proxy.js`

```javascript
const http = require('http');

const BOT_IP = '35.196.16.129';
const BOT_PORT = 8080;

module.exports = (req, res) => {
  // CORS headers
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
  res.setHeader('Content-Type', 'application/json');

  if (req.method === 'OPTIONS') {
    return res.status(200).end();
  }

  // Extract path and query
  let path = req.url.replace('/api/proxy', '');
  if (!path.startsWith('/api/')) {
    path = '/api' + path;
  }

  const query = path.includes('?') ? '?' + path.split('?')[1] : '';
  const cleanPath = path.split('?')[0];

  const url = `http://${BOT_IP}:${BOT_PORT}${cleanPath}${query}`;

  const request = http.get(url, { timeout: 10000 }, (response) => {
    let data = '';
    response.on('data', chunk => data += chunk);
    response.on('end', () => {
      res.status(response.statusCode).send(data);
    });
  });

  request.on('error', (err) => {
    res.status(503).json({
      error: 'Failed to reach bot API',
      details: err.message
    });
  });

  request.on('timeout', () => {
    request.destroy();
    res.status(504).json({ error: 'Bot API timeout' });
  });
};
```

### Step 3: Create `vercel.json`

```json
{
  "version": 2,
  "functions": {
    "api/proxy.js": {
      "maxDuration": 30
    }
  }
}
```

### Step 4: Deploy

```bash
vercel --prod
```

Then link your custom domain (same as Option 1, Step 6).

## Testing

After deployment and DNS propagation, test your endpoints:

```bash
# Replace with your domain
curl https://api.yourdomain.com/api/health

curl https://api.yourdomain.com/api/stats

curl https://api.yourdomain.com/api/sessions/latest
```

## Update Bot IP if VM IP changes

Edit `vercel.json` or `index.js` and change:
```javascript
const BOT_IP = 'NEW_IP_ADDRESS';
```

Then redeploy:
```bash
vercel --prod
```

## DNS Setup Example (for Namecheap/GoDaddy)

If Vercel gives you DNS records like:

```
Type: CNAME
Name: api
Value: cname.vercel.sh
```

Add to your registrar:
- **Type**: CNAME
- **Host**: api
- **Value**: cname.vercel.sh
- **TTL**: 3600

After 5-10 minutes:
```bash
nslookup api.yourdomain.com
# Should resolve to Vercel's IP
```

## Verify CORS is working

From browser console:
```javascript
fetch('https://api.yourdomain.com/api/stats')
  .then(r => r.json())
  .then(d => console.log(d))
  .catch(e => console.error(e));
```

Should return stats without CORS errors.

## Difference from Cloudflare Workers

| Feature | Cloudflare | Vercel |
|---------|-----------|--------|
| Cost | Free tier: 100K req/day | Free tier: 100 deployments/month, unlimited requests |
| Setup speed | 10 minutes | 5 minutes |
| Custom domain | Requires full domain migr. | Simple CNAME record |
| Latency | Edge (50ms avg) | US region (50-100ms) |
| Scalability | Global edge | US-based |
| Node.js support | Limited (module restrictions) | Full Node.js |

**Recommendation**: Use Vercel if you have a custom domain and want simplicity. Use Cloudflare Workers if you want global edge distribution.

## Next Steps

1. Deploy proxy to Vercel with custom domain
2. Test endpoints from browser
3. Create dashboard HTML file pointing to your custom domain instead of localhost
4. Host dashboard on Vercel, GitHub Pages, or your own server

