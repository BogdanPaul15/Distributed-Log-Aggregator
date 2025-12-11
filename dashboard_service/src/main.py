from fastapi import FastAPI, Depends, Request, Form, HTTPException
from fastapi.responses import HTMLResponse, RedirectResponse, Response
from fastapi.templating import Jinja2Templates
from sqlalchemy.orm import Session
from jose import jwt
import models, database, os, requests, json, csv, io, math
from urllib.parse import quote
from opensearchpy import OpenSearch
import datetime

app = FastAPI(title="Dashboard API")

try:
    models.Base.metadata.create_all(bind=database.engine)
except Exception as e:
    print(f"Warning: DB not ready yet. Retrying on next startup. Error: {e}")

templates = Jinja2Templates(directory="templates")

KEYCLOAK_URL = os.getenv("KEYCLOAK_URL")
CLIENT_ID = os.getenv("CLIENT_ID")
CLIENT_SECRET = os.getenv("CLIENT_SECRET")
APP_BASE_URL = "http://localhost:8000/"

OPENSEARCH_HOST = os.getenv("OPENSEARCH_HOST", "opensearch")
OPENSEARCH_PORT = int(os.getenv("OPENSEARCH_PORT", 9200))

client = OpenSearch(
    hosts=[{'host': OPENSEARCH_HOST, 'port': OPENSEARCH_PORT}],
    http_compress=True,
    use_ssl=False,
    verify_certs=False,
    ssl_assert_hostname=False,
    ssl_show_warn=False
)

def seed_logs():
    """Seed some mock logs if none exist."""
    try:
        if not client.indices.exists(index="app-logs-*"):
            print("Seeding mock logs...")
            mock_logs = [
                {"timestamp": "2025-11-28T10:00:01Z", "level": "INFO", "service": "payment-service", "message": "Transaction initiated for user 101"},
                {"timestamp": "2025-11-28T10:00:02Z", "level": "DEBUG", "service": "payment-service", "message": "Validating currency EUR"},
                {"timestamp": "2025-11-28T10:00:05Z", "level": "ERROR", "service": "checkout-service", "message": "Database connection timeout (5001ms)"},
                {"timestamp": "2025-11-28T10:02:10Z", "level": "WARN", "service": "inventory-service", "message": "Stock low for item #5521"},
                {"timestamp": "2025-11-28T10:05:00Z", "level": "INFO", "service": "auth-service", "message": "User testuser logged in successfully"},
                {"timestamp": "2025-11-28T10:06:23Z", "level": "FATAL", "service": "payment-gateway", "message": "Payment provider API unreachable"},
            ]
            
            dt = datetime.datetime.utcnow()
            index_name = f"app-logs-{dt.strftime('%Y.%m.%d')}"
            
            for log in mock_logs:
                client.index(index=index_name, body=log)
            
            client.indices.refresh(index=index_name)
            print(f"Seeded {len(mock_logs)} logs to {index_name}")
    except Exception as e:
        print(f"Warning: Could not seed logs: {e}")

@app.on_event("startup")
async def startup_event():
    seed_logs()

def get_logs(role: str, page: int = 1, size: int = 10, search_query: str = None, service: str = None, level: str = None, start_time: str = None, end_time: str = None):
    must_conditions = []
    
    if search_query:
        must_conditions.append({
            "multi_match": {
                "query": search_query,
                "fields": ["service", "level", "message"],
                "type": "phrase_prefix"
            }
        })

    if service:
        must_conditions.append({"term": {"service.keyword": service}})
        
    if level:
        must_conditions.append({"term": {"level.keyword": level}})

    timestamp_range = {}
    if start_time:
        timestamp_range["gte"] = start_time
    if end_time:
        timestamp_range["lte"] = end_time
        
    if role == 'viewer':
        must_conditions.append({"terms": {"level.keyword": ["INFO", "WARN"]}})
        # Viewer is restricted to last 3 hours. 
        # If user provides a start_time, it must be within the last 3 hours.
        # We can just add another range condition, OpenSearch will intersect them.
        must_conditions.append({"range": {"timestamp": {"gte": "now-3h"}}})
    
    if timestamp_range:
        must_conditions.append({"range": {"timestamp": timestamp_range}})
    
    base_query = {"match_all": {}}
    if must_conditions:
        base_query = {
            "bool": {
                "must": must_conditions
            }
        }

    query = {
        "from": (page - 1) * size,
        "size": size,
        "sort": [{"timestamp": "desc"}],
        "query": base_query,
        "track_total_hits": True
    }
        
    try:
        response = client.search(
            body=query,
            index="app-logs-*"
        )
        hits = response['hits']['hits']
        total_hits = response['hits']['total']['value']
        logs = [hit['_source'] for hit in hits]
        return logs, total_hits
    except Exception as e:
        print(f"Error fetching logs: {e}")
        return [], 0

@app.get("/export")
async def export_logs(
    request: Request, 
    format: str = "csv", 
    q: str = None,
    service: str = None,
    level: str = None,
    start_time: str = None,
    end_time: str = None
):
    token = request.cookies.get("access_token") or request.cookies.get("id_token")
    if not token:
        raise HTTPException(status_code=401, detail="Not authenticated")
    
    try:
        decoded = jwt.decode(token, None, options={"verify_signature": False, "verify_aud": False})
        kc_roles = decoded.get("realm_access", {}).get("roles", [])
        
        can_export = "admin" in kc_roles or "developer" in kc_roles
        if not can_export:
             raise HTTPException(status_code=403, detail="Insufficient permissions to export logs")
        
        # Export all matching logs
        logs, _ = get_logs(
            "admin", 
            page=1, 
            size=10000, 
            search_query=q,
            service=service,
            level=level,
            start_time=start_time,
            end_time=end_time
        )
        
        if format == "json":
            return Response(content=json.dumps(logs, indent=2), media_type="application/json", headers={"Content-Disposition": "attachment; filename=logs.json"})
        else:
            output = io.StringIO()
            writer = csv.writer(output)
            writer.writerow(["Timestamp", "Level", "Service", "Message"])
            for log in logs:
                writer.writerow([log.get("timestamp"), log.get("level"), log.get("service"), log.get("message")])
            
            return Response(content=output.getvalue(), media_type="text/csv", headers={"Content-Disposition": "attachment; filename=logs.csv"})
            
    except HTTPException as he:
        raise he
    except Exception as e:
        print(f"Export Error: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/", response_class=HTMLResponse)
async def read_root(
    request: Request, 
    db: Session = Depends(database.get_db),
    page: int = 1,
    size: int = 10,
    q: str = None,
    service: str = None,
    level: str = None,
    start_time: str = None,
    end_time: str = None
):
    token = request.cookies.get("access_token") or request.cookies.get("id_token")
    if token:
        try:
            decoded_payload = jwt.decode(token, None, options={
                "verify_signature": False,
                "verify_aud": False
            })
            username = decoded_payload.get("preferred_username")
            kc_roles = decoded_payload.get("realm_access", {}).get("roles", [])
            
            app_role = "viewer"
            if "admin" in kc_roles:
                app_role = "admin"
            elif "developer" in kc_roles:
                app_role = "developer"
            
            user_db = db.query(models.UserProfile).filter(models.UserProfile.username == username).first()
            if not user_db:
                user_db = models.UserProfile(username=username, role=app_role)
                db.add(user_db)
                db.commit()
            elif user_db.role != app_role:
                user_db.role = app_role
                db.commit()
            
            displayed_logs, total_hits = get_logs(
                app_role, 
                page=page, 
                size=size, 
                search_query=q,
                service=service,
                level=level,
                start_time=start_time,
                end_time=end_time
            )
            
            total_pages = math.ceil(total_hits / size)
            pretty_payload = json.dumps(decoded_payload, indent=2)
            
            return templates.TemplateResponse("dashboard.html", {
                "request": request, 
                "user": username, 
                "role": app_role,
                "token": token,
                "decoded_token": pretty_payload,
                "logs": displayed_logs,
                "page": page,
                "size": size,
                "total_pages": total_pages,
                "total_hits": total_hits,
                "q": q,
                "service": service,
                "level": level,
                "start_time": start_time,
                "end_time": end_time
            })
        except Exception as e:
            print(f"Session restore failed: {e}")
            pass

    return templates.TemplateResponse("dashboard.html", {"request": request, "user": None})

@app.get("/logout")
async def logout(request: Request):
    """Redirects to Keycloak to end the session."""
    public_keycloak_url = KEYCLOAK_URL.replace("http://keycloak:8080", "http://localhost:8080")
    
    encoded_redirect = quote(APP_BASE_URL)
    
    id_token = request.cookies.get("id_token")
    
    logout_url = f"{public_keycloak_url}/protocol/openid-connect/logout?post_logout_redirect_uri={encoded_redirect}&client_id={CLIENT_ID}"
    
    if id_token:
        logout_url += f"&id_token_hint={id_token}"
        
    response = RedirectResponse(logout_url)
    response.delete_cookie("id_token")
    response.delete_cookie("access_token")
    return response

@app.post("/login", response_class=HTMLResponse)
async def login(request: Request, username: str = Form(...), password: str = Form(...), db: Session = Depends(database.get_db)):
    token_url = f"{KEYCLOAK_URL}/protocol/openid-connect/token"
    payload = {
        "client_id": CLIENT_ID,
        "client_secret": CLIENT_SECRET,
        "username": username,
        "password": password,
        "grant_type": "password"
    }
    
    try:
        response = requests.post(token_url, data=payload)
        response.raise_for_status()
        token_data = response.json()
        access_token = token_data.get("access_token")
        id_token = token_data.get("id_token")
        
        decoded_payload = jwt.decode(access_token, None, options={
            "verify_signature": False,
            "verify_aud": False
        })
        
        pretty_payload = json.dumps(decoded_payload, indent=2)

        kc_roles = decoded_payload.get("realm_access", {}).get("roles", [])
        
        app_role = "viewer"
        if "admin" in kc_roles:
            app_role = "admin"
        elif "developer" in kc_roles:
            app_role = "developer"

        user_db = db.query(models.UserProfile).filter(models.UserProfile.username == username).first()
        if not user_db:
            user_db = models.UserProfile(username=username, role=app_role)
            db.add(user_db)
        else:
            user_db.role = app_role
            
        db.commit()

        page = 1
        size = 10
        displayed_logs, total_hits = get_logs(user_db.role, page=page, size=size)
        total_pages = math.ceil(total_hits / size)

        response = templates.TemplateResponse("dashboard.html", {
            "request": request, 
            "user": username, 
            "role": user_db.role,
            "token": access_token,
            "decoded_token": pretty_payload,
            "logs": displayed_logs,
            "page": page,
            "size": size,
            "total_pages": total_pages,
            "total_hits": total_hits,
            "q": None,
            "service": None,
            "level": None,
            "start_time": None,
            "end_time": None
        })
        
        if id_token:
            response.set_cookie(key="id_token", value=id_token, httponly=True)
        if access_token:
            response.set_cookie(key="access_token", value=access_token, httponly=True)
            
        return response
        
    except Exception as e:
        print(f"Login Error: {e}")
        return templates.TemplateResponse("dashboard.html", {
            "request": request, 
            "error": f"Login failed: {str(e)}"
        })