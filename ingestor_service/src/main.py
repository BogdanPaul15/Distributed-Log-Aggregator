from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import Optional
import datetime

app = FastAPI(title="Log Ingestor Service")

class LogEntry(BaseModel):
    service: str
    level: str
    message: str
    trace_id: Optional[str] = None
    timestamp: Optional[str] = None

@app.post("/ingest")
async def ingest_log(log: LogEntry):
    # In Milestone 3, this will push to Kafka 
    # For Milestone 2, we validate and acknowledge
    if log.level not in ["DEBUG", "INFO", "WARN", "ERROR", "FATAL"]:
        raise HTTPException(status_code=400, detail="Invalid log level")
    
    print(f"[{datetime.datetime.now()}] Received log from {log.service}: {log.message}")
    return {"status": "accepted", "ingestion_id": "mock-uuid-1234"}

@app.get("/health")
def health_check():
    return {"status": "operational", "replica": "Check Docker Hostname"}