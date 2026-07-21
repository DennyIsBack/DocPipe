using System.Text.Json;
using DocPipe.Application.Abstractions;
using DocPipe.Domain;
using Microsoft.AspNetCore.Mvc;

namespace DocPipe.Api.Controllers;

[ApiController]
[Route("v1/jobs")]
public class JobsController : ControllerBase
{
    private readonly IJobRepository _repository;
    private readonly IJobCache _cache;

    public JobsController(IJobRepository repository, IJobCache cache)
    {
        _repository = repository;
        _cache = cache;
    }

    /// <summary>Status do job. Responde do cache; só cai no Postgres se der miss.</summary>
    [HttpGet("{id:guid}")]
    public async Task<IActionResult> GetStatus(Guid id, CancellationToken ct)
    {
        var cached = await _cache.GetStatusAsync(id, ct);
        if (cached is not null)
            return Ok(new { jobId = id, status = cached, source = "cache" });

        var job = await _repository.GetJobAsync(id, ct);
        if (job is null)
            return NotFound(new { error = "Job não encontrado." });

        // Repovoa o cache para as próximas consultas.
        await _cache.SetStatusAsync(id, job.Status, ct);

        return Ok(new
        {
            jobId = job.Id,
            status = job.Status,
            attempt = job.Attempt,
            error = job.Error,
            createdAt = job.CreatedAt,
            updatedAt = job.UpdatedAt,
            source = "db"
        });
    }

    /// <summary>Resultado da extração. 409 enquanto o job não terminou.</summary>
    [HttpGet("{id:guid}/result")]
    public async Task<IActionResult> GetResult(Guid id, CancellationToken ct)
    {
        var job = await _repository.GetJobAsync(id, ct);
        if (job is null)
            return NotFound(new { error = "Job não encontrado." });

        var result = await _repository.GetResultAsync(id, ct);
        if (result is null)
            return Conflict(new { error = "Resultado ainda não disponível.", status = job.Status });

        return Ok(new
        {
            jobId = id,
            payload = JsonDocument.Parse(result.PayloadJson).RootElement,
            overallConfidence = result.OverallConfidence,
            needsReview = result.NeedsReview,
            createdAt = result.CreatedAt
        });
    }

    /// <summary>Fila de revisão humana: jobs de baixa confiança.</summary>
    [HttpGet]
    public async Task<IActionResult> List(
        [FromQuery] string status = JobStatus.NeedsReview,
        [FromQuery] int limit = 50,
        CancellationToken ct = default)
    {
        if (!JobStatus.All.Contains(status))
            return BadRequest(new { error = $"Status inválido: {status}.", allowed = JobStatus.All });

        var jobs = await _repository.ListByStatusAsync(status, Math.Clamp(limit, 1, 200), ct);

        return Ok(new
        {
            count = jobs.Count,
            jobs = jobs.Select(j => new { jobId = j.Id, j.Status, j.CreatedAt, j.UpdatedAt })
        });
    }
}
