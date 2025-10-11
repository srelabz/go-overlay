# Teste de Graceful Shutdown - Go Overlay

Este diretório contém tudo necessário para testar o sistema de graceful shutdown do Go Overlay.

## Serviços de Teste

O container de teste inclui:

1. **nginx** - Servidor web na porta 80
   - Pre-script: `/scripts/nginx-pre.sh`
   - Post-script: `/scripts/nginx-post.sh`

2. **test-service** - Serviço de teste que depende do nginx
   - Dependência: `nginx`
   - Aguarda 3 segundos após nginx iniciar

3. **monitor** - Serviço de monitoramento
   - Dependência: `test-service` 
   - Aguarda 2 segundos após test-service iniciar

4. **logger** - Serviço de logging independente
   - Sem dependências

## Como Testar

### Teste Interativo
```bash
./test-container.sh
```
- Inicia o container interativamente
- Pressione `Ctrl+C` para testar o graceful shutdown
- Nginx estará disponível em `http://localhost:8080`

### Teste Automatizado
```bash
./test-graceful-shutdown.sh
```
- Executa teste completo automaticamente
- Verifica se nginx está respondendo
- Envia SIGTERM para testar graceful shutdown
- Mostra logs do processo

### Teste Manual

1. **Construir imagem:**
   ```bash
   docker build -t go-overlay-test .
   ```

2. **Executar container:**
   ```bash
   docker run --rm -p 8080:80 --name tm-test go-overlay-test
   ```

3. **Testar nginx (em outro terminal):**
   ```bash
   curl http://localhost:8080
   curl http://localhost:8080/health
   ```

4. **Testar graceful shutdown:**
   ```bash
   docker kill --signal=SIGTERM tm-test
   ```

## O que Observar

Durante o shutdown graceful, você deve ver:

1. **Recebimento do sinal:** Mensagem indicando que SIGTERM foi recebido
2. **Ordem de parada:** Serviços sendo parados na ordem correta
3. **Timeouts:** Serviços que não respondem sendo forçadamente terminados após 10s
4. **Cleanup:** PTYs sendo fechados e recursos liberados
5. **Finalização:** Mensagem de "Graceful shutdown completed"

## Estrutura dos Arquivos

- `Dockerfile` - Configuração do container de teste
- `services.toml` - Configuração dos serviços
- `test-container.sh` - Script para teste interativo
- `test-graceful-shutdown.sh` - Script para teste automatizado
- `service-manager` - Binário compilado do orquestrador

## Logs Esperados

```
Go Overlay - Version: v0.1.0
[INFO] Loading services from /services.toml
[INFO] | === PRE-SCRIPT START --- [SERVICE: nginx] === |
Pre-script for nginx executed
Nginx pre-script completed
[INFO] | === PRE-SCRIPT END --- [SERVICE: nginx] === |
[INFO] Starting service: nginx
[INFO] Service nginx started successfully (PID: 123)
[nginx] nginx: [alert] low worker processes
...
```

Durante o shutdown:
```
[INFO] Received signal: terminated
[INFO] Initiating graceful shutdown...
[INFO] Starting graceful shutdown process...
[INFO] Gracefully stopping service: nginx
[INFO] Service nginx stopped gracefully
[INFO] All services stopped gracefully
[INFO] Graceful shutdown completed
``` 