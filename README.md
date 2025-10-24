# Good Telemetry

A web application for evaluating the quality of Prometheus metrics using LLM-powered analysis.

## Overview

Submit Prometheus time series data and receive detailed feedback on:
- Metric naming conventions (snake_case, proper suffixes)
- Label structure and usage
- Cardinality analysis and resource impact (memory estimation)
- Best practice adherence

Eventually will expand to support log evaluation as well.

## Features

- **Prometheus Metric Parser**: Parses standard Prometheus exposition format
- **Cardinality Calculator**: Estimates time series cardinality and memory usage based on [robustperception.io formulas](https://www.robustperception.io/how-much-ram-does-prometheus-2-x-need-for-cardinality-and-ingestion/)
- **High-Cardinality Detection**: Identifies problematic labels (user_id, email, timestamps, etc.)
- **LLM-Powered Analysis**: Uses Ollama for intelligent metric evaluation
- **htmx UI**: Fast, interactive web interface
- **Showcase Examples**: Hardcoded examples showing good and bad metrics

## Quick Start

### Prerequisites

- Go 1.25+
- [Ollama](https://ollama.ai) installed (local or remote)

### Quick Test (Local Ollama)

The fastest way to test the application:

1. **Install and start Ollama**:
```bash
# Install Ollama
curl https://ollama.ai/install.sh | sh

# Pull a small model for testing (llama3.2 is ~2GB)
ollama pull llama3.2:3b

# Start Ollama (it runs on localhost:11434 by default)
ollama serve
```

2. **In a new terminal, build and run the web server**:
```bash
go build -o bin/web ./cmd/web
export OLLAMA_MODEL=llama3.2:3b
./bin/web
```

3. **Test it**:
   - Open `http://localhost:8080`
   - Paste a metric:
   ```
   api_response_time{user_id="12345", endpoint="/profile"} 0.234
   ```
   - Click "Evaluate Metrics"
   - Watch the LLM identify the high-cardinality `user_id` label

### Setup for Remote GPU Server

If you want to use a separate GPU Linode:

1. On the GPU server:
```bash
# Install Ollama
curl https://ollama.ai/install.sh | sh

# Pull a larger model for better analysis
ollama pull llama3.1:8b

# Start Ollama and expose it
OLLAMA_HOST=0.0.0.0:11434 ollama serve
```

2. On your web server:
```bash
go build -o bin/web ./cmd/web
export LLM_BACKEND_URL=http://your-gpu-server:11434
export OLLAMA_MODEL=llama3.1:8b
./bin/web
```

3. Open browser to `http://localhost:8080`

## Configuration

Environment variables:

- `LLM_BACKEND_URL`: Ollama API endpoint (default: `http://localhost:11434`)
- `OLLAMA_MODEL`: Model to use (default: `llama2`)
- `WEB_PORT`: Web server port (default: `8080`)

See `config.example.env` for full configuration options.

## Usage

1. Paste one or more Prometheus metrics into the text box:
```
http_requests_total{method="GET", status="200"} 1027
```

2. Click "Evaluate Metrics"

3. View the analysis including:
   - Overall verdict (Good/Needs Improvement/Poor)
   - Specific issues found
   - Cardinality and memory estimates
   - Recommendations for improvement
   - Improved example

## Architecture

- **Web Server**: Go + Gin + htmx
- **LLM Backend**: Ollama (local or remote GPU server)
- **Cardinality Analysis**: Built-in Go calculator
- **Storage**: Hardcoded showcase examples (no database)

## Documentation

See [PLAN.md](PLAN.md) for detailed project plan and feature roadmap.

## Project Structure

```
.
├── cmd/
│   └── web/          # Web server entry point
├── internal/
│   ├── handlers/     # HTTP request handlers
│   ├── metrics/      # Metric parser
│   ├── cardinality/  # Cardinality calculator
│   └── llm/          # Ollama client
├── web/
│   ├── templates/    # HTML templates
│   └── static/       # CSS, JS
├── docs/             # RAG knowledge base (future)
└── examples/         # Good metric examples (future)
```
