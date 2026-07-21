FROM mcr.microsoft.com/dotnet/sdk:8.0 AS build
WORKDIR /src

# Restore antes do código: aproveita o cache enquanto só os .cs mudam.
COPY DocPipe.sln ./
COPY DocPipe.Api/*.csproj            DocPipe.Api/
COPY DocPipe.Application/*.csproj    DocPipe.Application/
COPY DocPipe.Domain/*.csproj         DocPipe.Domain/
COPY DocPipe.Infrastructure/*.csproj DocPipe.Infrastructure/
RUN dotnet restore

COPY . .
RUN dotnet publish DocPipe.Api/DocPipe.Api.csproj -c Release -o /app --no-restore

FROM mcr.microsoft.com/dotnet/aspnet:8.0 AS final
WORKDIR /app
COPY --from=build /app .
EXPOSE 8080
USER app
ENTRYPOINT ["dotnet", "DocPipe.Api.dll"]
