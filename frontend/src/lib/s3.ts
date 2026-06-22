import { S3Client, ListObjectsV2Command, GetObjectCommand, HeadObjectCommand } from '@aws-sdk/client-s3';
import { getSignedUrl } from '@aws-sdk/s3-request-presigner';

// To support both Node/Deno (process.env/Deno.env) and Cloudflare Workers (c.env)
export function getS3Client(env: Record<string, string | undefined>) {
  const region = env.S3_REGION || 'auto';
  const endpoint = env.S3_ENDPOINT;
  const accessKeyId = env.S3_ACCESS_KEY_ID || '';
  const secretAccessKey = env.S3_SECRET_ACCESS_KEY || '';

  return new S3Client({
    region,
    endpoint,
    credentials: {
      accessKeyId,
      secretAccessKey,
    },
    // Force path style for S3 compatible storage (like Minio or R2 if needed)
    forcePathStyle: env.S3_FORCE_PATH_STYLE === 'true', 
  });
}

export async function listDirectory(client: S3Client, bucket: string, prefix: string) {
  // Ensure prefix ends with a slash if it's not empty, to correctly list a directory
  const queryPrefix = prefix && !prefix.endsWith('/') ? prefix + '/' : prefix;

  const command = new ListObjectsV2Command({
    Bucket: bucket,
    Prefix: queryPrefix,
    Delimiter: '/', // Delimiter '/' groups objects into "directories" (CommonPrefixes)
  });

  const response = await client.send(command);

  const folders = (response.CommonPrefixes || []).map((p) => ({
    name: p.Prefix?.substring(queryPrefix.length).replace(/\/$/, '') || '',
    path: p.Prefix || '',
  }));

  const files = (response.Contents || [])
    .filter((obj) => obj.Key !== queryPrefix) // Exclude the directory itself if it exists as an object
    .map((obj) => ({
      name: obj.Key?.substring(queryPrefix.length) || '',
      path: obj.Key || '',
      size: obj.Size || 0,
      lastModified: obj.LastModified?.toISOString() || '',
    }));

  return { folders, files };
}

export async function getObjectInfo(client: S3Client, bucket: string, key: string) {
  const command = new HeadObjectCommand({ Bucket: bucket, Key: key });
  try {
    const response = await client.send(command);
    return {
      size: response.ContentLength || 0,
      contentType: response.ContentType || 'application/octet-stream',
      lastModified: response.LastModified?.toISOString() || '',
      eTag: response.ETag || '',
    };
  } catch (error: any) {
    if (error.name === 'NotFound' || error.$metadata?.httpStatusCode === 404) {
      return null;
    }
    throw error;
  }
}

export async function getObject(client: S3Client, bucket: string, key: string, range?: string) {
  const command = new GetObjectCommand({ 
    Bucket: bucket, 
    Key: key,
    ...(range ? { Range: range } : {})
  });
  const response = await client.send(command);
  return response;
}

/** Search all objects in the bucket, returning matching files and folder segments. */
export async function searchBucket(
  client: S3Client,
  bucket: string,
  query: string,
  maxKeys = 500,
) {
  const lowerQ = query.toLowerCase();
  const files: Array<{ name: string; path: string; size: number; lastModified: string }> = [];
  const folderSet = new Map<string, string>(); // path → name

  let continuationToken: string | undefined;

  do {
    const command = new ListObjectsV2Command({
      Bucket: bucket,
      MaxKeys: 1000,
      ContinuationToken: continuationToken,
    });
    const response = await client.send(command);

    for (const obj of response.Contents || []) {
      const key = obj.Key || '';
      const segments = key.split('/');

      // Check each intermediate folder segment
      for (let i = 0; i < segments.length - 1; i++) {
        const seg = segments[i];
        if (seg && seg.toLowerCase().includes(lowerQ)) {
          const folderPath = segments.slice(0, i + 1).join('/') + '/';
          if (!folderSet.has(folderPath)) folderSet.set(folderPath, seg);
        }
      }

      // Check filename
      const name = segments[segments.length - 1];
      if (name && name.toLowerCase().includes(lowerQ)) {
        files.push({
          name,
          path: key,
          size: obj.Size || 0,
          lastModified: obj.LastModified?.toISOString() || '',
        });
      }

      if (files.length >= maxKeys) break;
    }

    continuationToken = response.IsTruncated ? response.NextContinuationToken : undefined;
  } while (continuationToken && files.length < maxKeys);

  const folders = Array.from(folderSet.entries()).map(([path, name]) => ({ name, path }));
  return { files, folders };
}

/** Generate a presigned GET URL for an object in S3, valid for a specified duration. */
export async function getPresignedUrl(
  client: S3Client,
  bucket: string,
  key: string,
  expiresIn = 3600,
) {
  const command = new GetObjectCommand({ Bucket: bucket, Key: key });
  return getSignedUrl(client, command, { expiresIn });
}


