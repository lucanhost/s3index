# S3 CORS Configuration Guide

This guide explains how to configure CORS (Cross-Origin Resource Sharing) for S3 buckets to enable file preview functionality in S3 Index.

## Why CORS is Needed

When S3 Index serves the frontend from your server (e.g., `https://files.example.com`) but presigned URLs point directly to S3 (e.g., `https://bucket.s3.amazonaws.com`), browsers block cross-origin requests for security. This prevents:

- Image preview from loading
- Video/audio player from working
- PDF viewer from displaying
- Text/code files from being fetched

## AWS S3 CORS Configuration

### Step 1: Open AWS S3 Console
Go to https://s3.console.aws.amazon.com/

### Step 2: Select Your Bucket
Click on the bucket you're using with S3 Index.

### Step 3: Go to Permissions Tab
Click the "Permissions" tab, then scroll to "Cross-origin resource sharing (CORS)"

### Step 4: Edit CORS Configuration
Add the following CORS policy:

```json
[
  {
    "Sid": "S3IndexPreview",
    "AllowedOrigins": [
      "*"
    ],
    "AllowedMethods": [
      "GET",
      "HEAD"
    ],
    "AllowedHeaders": [
      "*"
    ],
    "ExposeHeaders": [
      "ETag",
      "x-amz-meta-content-type",
      "x-amz-meta-uri"
    ],
    "MaxAgeSeconds": 3000
  }
]
```

For production, replace `"*"` with your specific domain:
```json
[
  {
    "Sid": "S3IndexPreview",
    "AllowedOrigins": [
      "https://files.example.com"
    ],
    ...
  }
]
```

### Step 5: Save Changes
Click "Save changes" - CORS takes effect immediately.

## Cloudflare R2 CORS Configuration

### Using wrangler CLI

Add to your `wrangler.json` or `wrangler.toml`:

```json
{
  "bindings": [
    {
      "name": "BUCKET",
      "type": "r2_bucket",
      "bucket_name": "your-bucket-name"
    }
  ]
}
```

Then configure CORS in your Worker:

```javascript
export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    if (url.pathname.startsWith('/cors/')) {
      // Handle CORS preflight
      if (request.method === 'OPTIONS') {
        return new Response(null, {
          headers: {
            'Access-Control-Allow-Origin': '*',
            'Access-Control-Allow-Methods': 'GET, HEAD',
            'Access-Control-Allow-Headers': '*',
            'Access-Control-Max-Age': '3000',
          },
        });
      }
    }

    // ... rest of handler
  },
};
```

Or use R2's CORS via the dashboard:
1. Go to R2 Browser in Cloudflare dashboard
2. Select your bucket
3. Go to "Settings" tab
4. Under "CORS policy", add the same configuration as AWS S3

## MinIO CORS Configuration

### Using MinIO Client (mc)

```bash
# Install mc if not already
curl -O https://dl.min.io/client/mc/release/linux-arm64/mc
chmod +x mc

# Configure mc
./mc alias set myminio http://localhost:9000 ACCESSKEY SECRETKEY

# Set CORS
./mc admin cors add myminio/your-bucket < cors.json
```

Create `cors.json`:
```json
[
  {
    "AllowedOrigins": ["*"],
    "AllowedMethods": ["GET", "HEAD"],
    "AllowedHeaders": ["*"],
    "ExposeHeaders": ["ETag"],
    "MaxAgeSeconds": 3000
  }
]
```

### Using MinIO Console
1. Open MinIO browser at `http://localhost:9000`
2. Click on your bucket
3. Go to "Permissions" tab
4. Under "CORS Settings", add:
   - Allowed Origins: `*`
   - Allowed Methods: `GET, HEAD`
   - Allowed Headers: `*`
   - Expose Headers: `ETag`
   - Max Age: `3000`

## Testing CORS Configuration

### Test with curl
After setting CORS, test with a presigned URL:

```bash
# Get a presigned URL (via your app)
URL="https://your-bucket.s3.amazonaws.com/some/file.jpg?X-Amz-..."

# Check CORS headers
curl -I -H "Origin: http://localhost:5173" "$URL" | grep -i "access-control"
```

You should see:
```
Access-Control-Allow-Origin: *
Access-Control-Expose-Headers: ETag
```

### Test in Browser
1. Open S3 Index in your browser
2. Click on an image/video/PDF file
3. The preview should load without errors in DevTools Console

Check DevTools:
- Open Developer Tools (F12)
- Go to Console tab
- Look for CORS errors like:
  ```
  Access to fetch at 'https://...' from origin 'http://...' has been blocked
  ```

## Troubleshooting

### Still Seeing CORS Errors?
1. **Wait 5-10 minutes** - Some S3 regions take time to propagate
2. **Check the exact domain** - Make sure your S3 Index URL matches what's in `AllowedOrigins`
3. **Verify presigned URLs** - Ensure URLs use the correct endpoint format

### For Cloudflare R2:
- If using custom domain, ensure the CNAME points to the correct R2 endpoint
- Check that your Worker includes CORS headers in responses

### For MinIO:
- Ensure you're using path-style or virtual-hosted-style URLs correctly
- If using `S3_FORCE_PATH_STYLE=true`, CORS should still work

## Security Considerations

For production, restrict `AllowedOrigins`:
```json
{
  "AllowedOrigins": ["https://your-domain.com"],
  "AllowedMethods": ["GET", "HEAD"],
  "AllowedHeaders": ["Range"],
  "ExposeHeaders": ["Content-Length", "Content-Range"],
  "MaxAgeSeconds": 3600
}
```

The `Range` header is needed for video seeking and PDF partial loads.