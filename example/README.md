# QUIC-Go Examples

Esta pasta contém exemplos completos demonstrando diferentes aspectos do quic-go, incluindo HTTP/3, QUIC básico, e recursos L4S.

## Estrutura dos Exemplos

```
example/
├── README.md           # Este arquivo
├── main.go            # Servidor HTTP/3 completo
├── client/            # Cliente HTTP/3
│   └── main.go
├── echo/              # Exemplo QUIC básico (server/client)
│   └── echo.go
├── l4s-config/        # Exemplo de configuração L4S
│   ├── README.md
│   └── main.go
└── l4s-echo/          # Exemplo L4S com métricas Prometheus
    ├── README.md
    ├── server.go
    ├── metrics.go
    └── client/
        └── client.go
```

## Guia Rápido

### 1. Servidor HTTP/3 Completo (`main.go`)

**O que faz:** Servidor HTTP/3 com vários endpoints de teste

```bash
# Executar servidor básico
go run example/main.go

# Com configurações personalizadas
go run example/main.go -bind localhost:8443 -tcp
```

**Endpoints disponíveis:**
- `/{número}` - Gera dados do tamanho especificado
- `/demo/echo` - Ecoa dados enviados via POST
- `/demo/upload` - Upload de arquivos
- `/demo/tiles` - Página com múltiplas imagens

### 2. Cliente HTTP/3 (`client/`)

**O que faz:** Cliente para testar servidores HTTP/3

```bash
# Teste básico
go run example/client/main.go https://localhost:6121/1000

# Com opções avançadas
go run example/client/main.go -keylog keys.log -insecure https://localhost:6121/demo/echo
```

### 3. QUIC Echo (`echo/`)

**O que faz:** Exemplo básico de comunicação QUIC (sem HTTP)

```bash
# Executa servidor e cliente automaticamente
go run example/echo/echo.go
```

### 4. L4S Configuration (`l4s-config/`)

**O que faz:** Demonstra configuração L4S com Prague congestion control

```bash
# Servidor L4S
go run example/l4s-config/main.go -enable-l4s

# Comparar com RFC9002
go run example/l4s-config/main.go -disable-l4s
```

### 5. L4S Echo com Métricas (`l4s-echo/`)

**O que faz:** Servidor e cliente L4S com métricas Prometheus completas

```bash
# Terminal 1: Servidor
go run example/l4s-echo/server.go example/l4s-echo/metrics.go

# Terminal 2: Cliente
cd example/l4s-echo/client && go run client.go
```

**Características:**
- Métricas Prague completas (alpha, CWND, ECN feedback)
- Flag `-enable-l4s` para controle de L4S
- Push para Prometheus opcional

## Cenários de Uso

### Teste de Performance HTTP/3

```bash
# Terminal 1: Servidor
go run example/main.go

# Terminal 2: Cliente
time go run example/client/main.go -q https://localhost:6121/10485760  # 10MB
```

### Comparação L4S vs RFC9002

```bash
# Terminal 1: Servidor L4S
go run example/l4s-config/main.go -enable-l4s

# Terminal 2: Teste L4S
go run example/client/main.go https://localhost:8443/1000

# Comparar com servidor padrão
go run example/main.go
go run example/client/main.go https://localhost:6121/1000
```

### Debugging e Análise

```bash
# Cliente com key logging para Wireshark
go run example/client/main.go -keylog keys.log https://localhost:8443/1000
```

### Teste de Multiplexação

```bash
# Servidor
go run example/main.go

# Browser: https://localhost:6121/demo/tiles
# Mostra 200 imagens carregadas simultaneamente via HTTP/3
```

## Configurações Avançadas

### Certificados Personalizados

```bash
# Todos os servidores suportam certificados próprios
go run example/main.go -cert mycert.pem -key mykey.pem
go run example/l4s-config/main.go -cert mycert.pem -key mykey.pem
```

### Portas Personalizadas

```bash
# Servidor HTTP/3
go run example/main.go -bind localhost:8443

# Servidor L4S
go run example/l4s-config/main.go -addr :9443
```

### Compatibilidade HTTP/1.1 e HTTP/2

```bash
# Habilitar suporte HTTP/1.1, HTTP/2 e HTTP/3
go run example/main.go -tcp

# Agora curl funciona normalmente
curl -k https://localhost:6121/1000
```

## Resolução de Problemas

### "Failed to connect"

**Causa:** curl não suporta HTTP/3 por padrão
**Solução:** Use o cliente quic-go ou browser

```bash
# Nao funciona
curl https://localhost:6121/1000

# Funciona
go run example/client/main.go https://localhost:6121/1000
```

### "Certificate not trusted"

**Solução:** Aceite o certificado auto-assinado

```bash
# Cliente
go run example/client/main.go -insecure https://localhost:6121/1000

# Curl (se usando -tcp)
curl -k https://localhost:6121/1000

# Browser: Clique em "Avançado" → "Prosseguir"
```

### "L4S can only be enabled when using Prague"

**Causa:** Configuração inválida de L4S
**Solução:** Sempre use Prague com L4S

```bash
# Incorreto
config.EnableL4S = true
config.CongestionControlAlgorithm = RFC9002

# Correto  
config.EnableL4S = true
config.CongestionControlAlgorithm = Prague
```

## Links Relacionados

- [L4S Configuration Examples](l4s-config/README.md)
- [L4S Troubleshooting Guide](../docs/l4s-troubleshooting.md)
- [Prague Algorithm Tuning](../docs/prague-algorithm-tuning.md)

## Execução Rápida

```bash
# Testar tudo rapidamente
cd /Users/juscelioreis/Documents/code/quic-go

# Servidor básico
go run example/main.go &

# Cliente teste
go run example/client/main.go https://localhost:6121/1000

# L4S
go run example/l4s-config/main.go -enable-l4s &
go run example/client/main.go https://localhost:8443/1000
```

Todos os exemplos são autocontidos e demonstram diferentes aspectos do QUIC e HTTP/3 com quic-go!