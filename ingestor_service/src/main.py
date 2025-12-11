from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import Optional
import datetime
import os
import time
import socket
from opensearchpy import OpenSearch

app = FastAPI(title="Log Ingestor Service")

OPENSEARCH_HOST = os.getenv("OPENSEARCH_HOST", "opensearch")
OPENSEARCH_PORT = int(os.getenv("OPENSEARCH_PORT", 9200))
HOSTNAME = socket.gethostname()

client = OpenSearch(
    hosts=[{'host': OPENSEARCH_HOST, 'port': OPENSEARCH_PORT}],
    http_compress=True,
    use_ssl=False,
    verify_certs=False,
    ssl_assert_hostname=False,
    ssl_show_warn=False
)


def init_opensearch():
    template_name = "app-logs-template"
    index_pattern = "app-logs-*"
    
    template_body = {
        "index_patterns": [index_pattern],
        "mappings": {
            "properties": {
                "timestamp": {"type": "date"},
                "level": {"type": "keyword"},
                "service": {"type": "keyword"},
                "message": {"type": "text"},
                "trace_id": {"type": "keyword"}
            }
        }
    }
    
    max_retries = 10
    for i in range(max_retries):
        try:
            if not client.indices.exists_template(name=template_name):
                client.indices.put_template(name=template_name, body=template_body)
                print(f"Created OpenSearch template: {template_name}")
            else:
                print(f"OpenSearch template {template_name} already exists.")
            return
        except Exception as e:
            print(f"Attempt {i+1}/{max_retries}: Could not initialize OpenSearch template: {e}")
            time.sleep(5)
    
    print("Failed to initialize OpenSearch template after multiple attempts.")

@app.on_event("startup")
async def startup_event():
    init_opensearch()

class LogEntry(BaseModel):
    service: str
    level: str
    message: str
    trace_id: Optional[str] = None
    timestamp: Optional[str] = None

@app.post("/ingest")
async def ingest_log(log: LogEntry):
    if log.level not in ["DEBUG", "INFO", "WARN", "ERROR", "FATAL"]:
        raise HTTPException(status_code=400, detail="Invalid log level")
    
    ts = log.timestamp if log.timestamp else datetime.datetime.utcnow().isoformat()
    
    try:
        dt_str = ts.replace('Z', '+00:00')
        dt = datetime.datetime.fromisoformat(dt_str)
    except ValueError:
        dt = datetime.datetime.utcnow()
        ts = dt.isoformat()

    index_name = f"app-logs-{dt.strftime('%Y.%m.%d')}"
    
    document = {
        "timestamp": ts,
        "level": log.level,
        "service": log.service,
        "message": log.message,
        "trace_id": log.trace_id,
        "ingested_by": HOSTNAME
    }
    
    try:
        response = client.index(
            index=index_name,
            body=document,
            refresh=True
        )
        print(f"[{datetime.datetime.now()}] [Replica: {HOSTNAME}] Indexed log to {index_name}: {log.message}")
        return {"status": "accepted", "ingestion_id": response["_id"], "index": index_name}
    except Exception as e:
        print(f"Error indexing log: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/health")
def health_check():
    return {"status": "operational", "replica": "Check Docker Hostname"}