from opentelemetry.propagate import extract

def extract_trace_context(carrier: dict):
    """
    Extracts W3C TraceContext from a dictionary (JSON payload).
    Returns an OpenTelemetry context that can be used to link spans.
    """
    if carrier is None:
        carrier = {}
    return extract(carrier)
