using Amazon.S3;
using Amazon.S3.Model;
using DocPipe.Application.Abstractions;
using Microsoft.Extensions.Options;

namespace DocPipe.Infrastructure.Storage;

/// <summary>
/// Object storage via API S3. Aponta para o MinIO local e para S3/Blob na cloud
/// sem mudança de código — só de endpoint.
/// </summary>
public class S3DocumentStorage : IDocumentStorage
{
    private readonly IAmazonS3 _s3;
    private readonly StorageOptions _options;

    public S3DocumentStorage(IAmazonS3 s3, IOptions<StorageOptions> options)
    {
        _s3 = s3;
        _options = options.Value;
    }

    public async Task<string> SaveAsync(
        Stream content, string storageKey, string contentType, CancellationToken ct = default)
    {
        await _s3.PutObjectAsync(new PutObjectRequest
        {
            BucketName = _options.Bucket,
            Key = storageKey,
            InputStream = content,
            ContentType = contentType,
            // O MinIO não calcula o checksum sozinho em stream não-seekable.
            AutoCloseStream = false
        }, ct);

        return storageKey;
    }
}
