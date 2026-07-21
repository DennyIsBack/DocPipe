using DocPipe.Application.Documents;
using Microsoft.AspNetCore.Mvc;

namespace DocPipe.Api.Controllers;

[ApiController]
[Route("v1/documents")]
public class DocumentsController : ControllerBase
{
    private static readonly string[] AllowedContentTypes =
    [
        "application/pdf", "image/jpeg", "image/png", "image/tiff"
    ];

    private readonly SubmitDocumentHandler _handler;
    private readonly IConfiguration _configuration;
    private readonly ILogger<DocumentsController> _logger;

    public DocumentsController(
        SubmitDocumentHandler handler,
        IConfiguration configuration,
        ILogger<DocumentsController> logger)
    {
        _handler = handler;
        _configuration = configuration;
        _logger = logger;
    }

    /// <summary>Recebe o documento, enfileira e devolve o jobId imediatamente.</summary>
    [HttpPost]
    [ProducesResponseType(StatusCodes.Status202Accepted)]
    [ProducesResponseType(StatusCodes.Status400BadRequest)]
    public async Task<IActionResult> Submit(
        IFormFile file,
        [FromQuery] string documentType = "invoice",
        CancellationToken ct = default)
    {
        if (file is null || file.Length == 0)
            return BadRequest(new { error = "Arquivo obrigatório." });

        var maxBytes = _configuration.GetValue("MaxUploadBytes", 20L * 1024 * 1024);
        if (file.Length > maxBytes)
            return BadRequest(new { error = $"Arquivo excede o limite de {maxBytes} bytes." });

        if (!AllowedContentTypes.Contains(file.ContentType))
            return BadRequest(new
            {
                error = $"Content-type não suportado: {file.ContentType}.",
                allowed = AllowedContentTypes
            });

        await using var stream = file.OpenReadStream();

        var jobId = await _handler.HandleAsync(new SubmitDocumentCommand(
            stream, file.FileName, file.ContentType, file.Length, documentType), ct);

        _logger.LogInformation("Job {JobId} enfileirado para {Filename}", jobId, file.FileName);

        return Accepted(new { jobId });
    }
}
