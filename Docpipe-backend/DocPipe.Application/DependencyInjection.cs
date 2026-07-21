using DocPipe.Application.Documents;
using Microsoft.Extensions.DependencyInjection;

namespace DocPipe.Application;

public static class DependencyInjection
{
    public static IServiceCollection AddApplication(this IServiceCollection services)
    {
        services.AddScoped<SubmitDocumentHandler>();
        return services;
    }
}
