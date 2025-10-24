# Good Telemetry

A web application for evaluating the quality of Prometheus metrics using LLM-powered analysis.

## Overview

Submit Prometheus time series data and receive detailed feedback on:
- Metric naming conventions
- Label structure and usage
- Cardinality analysis and resource impact
- Best practice adherence

Eventually will expand to support log evaluation as well.

## Documentation

See [PLAN.md](PLAN.md) for detailed project plan and feature set.

## Status

🚧 Project in early development

## Architecture

- **Web Server**: Go application (hosted on Linode)
- **LLM Backend**: Ollama on separate GPU Linode
- **RAG System**: Knowledge base of good metric examples and Prometheus documentation
