# Good Telemetry - Project Plan

## Overview
A web application for evaluating the quality of Prometheus metrics (and eventually logs) using LLM-powered analysis. Users submit time series data and receive detailed feedback on metric naming, label usage, cardinality, and structure.

## Architecture

### Frontend (Web Server - Linode)
- **Technology**: Go with standard library `net/http` or lightweight framework
- **Hosting**: Personal Linode server
- **Responsibilities**:
  - Serve web UI with input text box
  - Display example submissions and LLM responses
  - Handle user input validation
  - Forward evaluation requests to LLM backend
  - Store and display showcase examples

### Backend (LLM Service - Separate GPU Linode)
- **Technology**: Ollama running on GPU Linode
- **Model**: TBD (likely Llama 2/3 or similar)
- **Responsibilities**:
  - Receive metric evaluation requests
  - Perform RAG lookups against good metric examples
  - Generate detailed analysis and recommendations
  - Return structured evaluation results

### Data Storage
- **Examples Database**: Store user submissions and LLM responses for showcase
- **RAG Knowledge Base**: Collection of good metric examples, Prometheus docs, and PDFs
- **Format**: TBD (SQLite for simplicity or PostgreSQL for production)

## Core Features

### Phase 1: Prometheus Metrics Evaluation

#### Input Processing
- Accept single or multiple Prometheus time series entries
- Parse format: `metric_name{label1="value1", label2="value2"} value timestamp`
- Support various input formats (raw metrics, PromQL queries, metric samples)

#### Evaluation Criteria

**Metric Naming**
- Follow Prometheus naming conventions (snake_case, descriptive suffixes like _total, _bytes, etc.)
- Appropriate base units
- Clear and descriptive names
- Proper use of metric types (counter, gauge, histogram, summary)

**Label Structure**
- Appropriate label granularity
- Avoid high-cardinality labels (user IDs, email addresses, timestamps)
- Consistent label naming across metrics
- Proper use of label dimensions
- Detection of label explosion risks

**Cardinality Analysis**
- Calculate potential cardinality from label combinations
- **Memory estimation calculations** (similar to https://www.robustperception.io/how-much-ram-does-prometheus-2-x-need-for-cardinality-and-ingestion/)
  - Estimate RAM requirements based on active series
  - Calculate ingestion rate impact
  - Warn about resource implications
- Identify problematic label combinations
- Flag unbounded cardinality risks

**General Structure**
- Metric type appropriateness
- Consistency with similar metrics
- Completeness of metadata
- Adherence to best practices

#### LLM Analysis
- Send parsed metrics + RAG context to Ollama backend
- LLM evaluates against best practices
- Returns:
  - Overall quality score/verdict (Good/Needs Improvement/Poor)
  - Specific issues identified
  - Recommendations for improvement
  - Example of improved version
  - Cardinality and resource impact warnings

### Phase 2: Log Evaluation (Future)
- Extend system to evaluate log quality
- Structured vs unstructured log analysis
- Log level appropriateness
- Cardinality concerns in log labels/fields
- Resource impact of log volume

## RAG Knowledge Base

### Content Sources
- **Good Metric Examples**: Curated collection of well-structured Prometheus metrics
- **Prometheus Documentation**: Official docs on best practices, naming conventions
- **Blog Posts/PDFs**:
  - Robust Perception articles
  - Prometheus community best practices
  - Real-world case studies
  - Memory and resource calculation guides
- **Anti-patterns**: Examples of poor metrics and why they're problematic

### Implementation
- Vector embeddings of knowledge base
- Semantic search for relevant examples
- Context injection into LLM prompts

## User Interface

### Main Page
- **Input Section**:
  - Large text area for metric input
  - Support for single or multiple metrics
  - Optional: File upload for bulk metrics
  - Submit button

- **Results Section**:
  - Display LLM analysis
  - Highlight specific issues
  - Show recommendations
  - Display cardinality calculations and memory estimates
  - Provide improved examples

- **Showcase Section**:
  - Display recent evaluations from other users
  - Show variety of good and bad examples
  - Real LLM responses
  - Filter by quality rating or issue type

### Additional Pages
- **About**: Explanation of evaluation criteria
- **Best Practices**: Guide to good Prometheus metrics
- **Resources**: Links to documentation and tools

## API Design

### Web Server Endpoints
- `GET /` - Main UI page
- `POST /api/evaluate` - Submit metrics for evaluation
- `GET /api/showcase` - Retrieve showcase examples
- `GET /api/examples` - Get good metric examples

### LLM Backend Endpoints
- `POST /evaluate` - Receive metrics and return analysis
- `POST /rag/search` - Query knowledge base (if needed separately)

## Technical Implementation Details

### Go Project Structure
```
good_telemetry/
├── cmd/
│   ├── web/           # Web server application
│   └── llm/           # LLM backend service (Ollama wrapper)
├── internal/
│   ├── handlers/      # HTTP handlers
│   ├── metrics/       # Metric parsing and analysis
│   ├── cardinality/   # Cardinality calculation logic
│   ├── llm/           # LLM client and prompt management
│   ├── rag/           # RAG implementation
│   ├── storage/       # Database operations
│   └── templates/     # HTML templates
├── web/
│   ├── static/        # CSS, JS, images
│   └── templates/     # HTML templates
├── docs/              # RAG knowledge base documents
├── examples/          # Good metric examples for RAG
├── migrations/        # Database migrations
└── tests/
```

### Metric Parser
- Parse Prometheus exposition format
- Extract metric name, labels, value, timestamp
- Support for different input formats
- Validation of syntax

### Cardinality Calculator
- Calculate label combination permutations
- Estimate active time series count
- Memory usage estimation formulas
- Ingestion rate impact calculations
- Warning thresholds

### LLM Integration
- HTTP client to Ollama API
- Prompt engineering for consistent analysis
- Structured response parsing
- Error handling and retries
- Rate limiting

### RAG System
- Document embedding generation
- Vector similarity search
- Context window management
- Relevant example retrieval

## Deployment

### Web Server Linode
- Go binary deployment
- Systemd service
- Nginx reverse proxy (optional)
- SSL/TLS certificates
- Database (SQLite/PostgreSQL)

### GPU Linode
- Ollama installation
- Model downloads and configuration
- API endpoint exposure
- Resource monitoring

## Future Enhancements

### Metrics (Phase 1 Extensions)
- Batch evaluation mode
- Export evaluation reports
- Integration with Prometheus API to pull live metrics
- Historical trend analysis
- Team/organization accounts
- Custom evaluation rules

### Logs (Phase 2)
- Log format evaluation
- Structured logging best practices
- Field cardinality analysis
- Performance impact estimation
- Log sampling recommendations

### Additional Features
- API for CI/CD integration
- Browser extension for quick evaluation
- Slack/Discord bot integration
- Comparison mode (before/after refactoring)
- Learning mode (explain concepts interactively)

## Success Metrics
- Evaluation accuracy (subjective, based on expert review)
- User engagement (submissions, return visits)
- Performance (response time, LLM latency)
- Resource usage (memory, CPU on both Linodes)

## Open Questions
1. Which Ollama model to use? (Size vs accuracy tradeoff)
2. Database choice? (SQLite for simplicity vs PostgreSQL for features)
3. RAG implementation? (Custom vs library like langchain-go)
4. Authentication needed? (Public vs private use)
5. Rate limiting strategy?
6. Data retention policy for showcase examples?

## UI/UX Improvements Backlog

### High Priority

#### RAG Integration (Hard)
- **Importance**: Critical for evaluation quality
- **Description**: Implement RAG to reference actual Prometheus best practices and examples
- **Dependencies**: RAG system implementation, vector DB, document embedding
- **Impact**: Significantly improves LLM evaluation accuracy and usefulness

#### Share Link Feature (Hard)
- **Importance**: High - enables collaboration and sharing results
- **Description**: Generate short links after evaluation that include both input metrics and results
- **Implementation**:
  - Backend storage for evaluation results (DB or file-based)
  - Short ID generation (UUID or nanoid)
  - Route to retrieve and display shared evaluations (e.g., `/share/:id`)
  - UI button to copy shareable link after evaluation completes
- **Dependencies**: Database/storage layer
- **Notes**: Can share storage mechanism with evaluation history feature

#### Mobile Responsiveness (Medium)
- **Importance**: High - accessibility for all users
- **Description**: Ensure app works well on mobile devices
- **Tasks**:
  - Responsive textarea sizing
  - Touch-friendly button sizing
  - Proper viewport meta tags
  - Test on various screen sizes
  - Adjust layout for narrow screens

### Medium Priority

#### Error Handling UI (Medium)
- **Importance**: Medium - improves user experience
- **Description**: Clear visual feedback when errors occur
- **Tasks**:
  - Show validation errors before submission
  - Display API errors gracefully
  - Timeout handling for slow LLM responses
  - Network error handling

#### Better Placeholder/Example Text (Easy)
- **Importance**: Medium - helps users understand input format
- **Description**: Show complete, realistic example metric in textarea placeholder
- **Example**: `http_requests_total{method="GET", status="200", handler="/api/users"} 1234`

#### Visual Separation: Results vs Examples (Medium)
- **Importance**: Medium - reduces confusion
- **Description**: Clearly distinguish user evaluation results from example evaluations
- **Tasks**:
  - Different background colors or borders
  - Clear section headers
  - Scroll-to-results after submission
  - Maybe collapse examples when results are shown

#### Copy-to-Clipboard for Results (Easy)
- **Importance**: Medium - convenience feature
- **Description**: Add button to copy evaluation results to clipboard
- **Implementation**: Simple clipboard API, button next to results

### Low Priority

#### Add Favicon (Easy)
- **Importance**: Low - polish
- **Description**: Create and add favicon for browser tab
- **Format**: SVG or PNG in multiple sizes

#### Label for Dark Mode Toggle (Easy)
- **Importance**: Low - UX clarity
- **Description**: Add text label or tooltip for the moon icon
- **Current**: Just a moon emoji with no context

#### Better "Try Random Example" Integration (Medium)
- **Importance**: Low - UX polish
- **Description**: Improve positioning and styling of random example button
- **Ideas**:
  - Move inside textarea area (floating button)
  - Place directly below textarea
  - Add keyboard shortcut

#### Intentional Spacing/Hierarchy (Medium)
- **Importance**: Low - visual polish
- **Description**: Review and improve visual hierarchy and spacing throughout app
- **Tasks**:
  - Consistent padding/margins
  - Visual grouping of related elements
  - Typography hierarchy review

### Future Considerations

#### Evaluation History (Hard)
- **Description**: Store and display past evaluations for logged-in users
- **Dependencies**: Authentication system, database, share link storage mechanism
- **Notes**: Lower priority until user accounts are implemented
