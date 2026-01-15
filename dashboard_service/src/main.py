from fastapi import FastAPI, Depends, Request, Form, HTTPException
from fastapi.responses import HTMLResponse, RedirectResponse, Response
from fastapi.templating import Jinja2Templates
from sqlalchemy.orm import Session
from jose import jwt
import models, database, os, requests, json, csv, io, math
from urllib.parse import quote
from opensearchpy import OpenSearch
import datetime
from pydantic import BaseModel
from typing import Optional, List

app = FastAPI(title="Dashboard API")

# Retry logic for DB connection
import time
from sqlalchemy.exc import OperationalError

MAX_RETRIES = 15
RETRY_DELAY = 3

for i in range(MAX_RETRIES):
    try:
        models.Base.metadata.create_all(bind=database.engine)
        print("Database tables created successfully.")
        break
    except Exception as e:
        print(f"Database not ready (Attempt {i+1}/{MAX_RETRIES}). Retrying in {RETRY_DELAY} seconds... Error: {e}")
        time.sleep(RETRY_DELAY)
else:
    print("Could not connect to database after several attempts.")

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

def get_logs(role: str, page: int = 1, size: int = 10, search_query: str = None, service: str = None, level: str = None, start_time: str = None, end_time: str = None):
    must_conditions = []
    
    if search_query:
        must_conditions.append({
            "bool": {
                "should": [
                    {
                        "multi_match": {
                            "query": search_query,
                            "fields": ["message"],
                            "type": "phrase_prefix"
                        }
                    },
                    {
                        "multi_match": {
                            "query": search_query,
                            "fields": ["service", "level"]
                        }
                    }
                ],
                "minimum_should_match": 1
            }
        })

    if service:
        must_conditions.append({"term": {"service.keyword": service}})
        
    if level:
        must_conditions.append({"term": {"level.keyword": level}})

    timestamp_range = {}
    if start_time:

        try:
            dt = datetime.datetime.fromisoformat(start_time)
            dt_aware = dt.astimezone() 
            timestamp_range["gte"] = dt_aware.isoformat()
        except ValueError:
            timestamp_range["gte"] = start_time

    if end_time:
        try:
            dt = datetime.datetime.fromisoformat(end_time)
            dt_aware = dt.astimezone()
            timestamp_range["lte"] = dt_aware.isoformat()
        except ValueError:
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

def get_saved_searches_helper(db, username, roles):
    if "admin" in roles:
        return db.query(models.SavedSearch).all()
    return db.query(models.SavedSearch).filter(models.SavedSearch.username == username).all()

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
                "end_time": end_time,
                "saved_searches": get_saved_searches_helper(db, username, kc_roles)
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

@app.get("/login")
async def login_redirect():
    public_keycloak_url = KEYCLOAK_URL.replace("http://keycloak:8080", "http://localhost:8080")
    return RedirectResponse(
        f"{public_keycloak_url}/protocol/openid-connect/auth"
        f"?client_id={CLIENT_ID}"
        f"&response_type=code"
        f"&redirect_uri={quote(APP_BASE_URL + 'callback')}"
        f"&scope=openid"
    )

@app.get("/callback")
async def callback(request: Request, code: str, db: Session = Depends(database.get_db)):
    token_url = f"{KEYCLOAK_URL}/protocol/openid-connect/token"
    payload = {
        "grant_type": "authorization_code",
        "code": code,
        "redirect_uri": APP_BASE_URL + "callback",
        "client_id": CLIENT_ID,
        "client_secret": CLIENT_SECRET,
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
        
        kc_roles = decoded_payload.get("realm_access", {}).get("roles", [])
        username = decoded_payload.get("preferred_username")
        
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

        response = RedirectResponse(url="/")
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

# --- Saved Search API ---

class SavedSearchBase(BaseModel):
    name: str
    query: str

class SavedSearchCreate(SavedSearchBase):
    pass

class SavedSearchUpdate(BaseModel):
    name: Optional[str] = None
    query: Optional[str] = None

class SavedSearchResponse(SavedSearchBase):
    id: int
    username: str
    created_at: datetime.datetime
    updated_at: datetime.datetime

    class Config:
        orm_mode = True

def get_current_user_from_token(request: Request):
    """Helper to extract user info from token in cookies"""
    token = request.cookies.get("access_token") or request.cookies.get("id_token")
    if not token:
        raise HTTPException(status_code=401, detail="Not authenticated")
    try:
        decoded = jwt.decode(token, None, options={"verify_signature": False, "verify_aud": False})
        return {
            "username": decoded.get("preferred_username"),
            "roles": decoded.get("realm_access", {}).get("roles", [])
        }
    except Exception:
        raise HTTPException(status_code=401, detail="Invalid token")

@app.post("/searches", response_model=SavedSearchResponse)
async def create_saved_search(
    search: SavedSearchCreate,
    request: Request,
    db: Session = Depends(database.get_db)
):
    user_info = get_current_user_from_token(request)
    username = user_info["username"]

    db_search = models.SavedSearch(
        name=search.name,
        query=search.query,
        username=username
    )
    db.add(db_search)
    db.commit()
    db.refresh(db_search)
    return db_search

@app.get("/searches", response_model=List[SavedSearchResponse])
async def list_saved_searches(
    request: Request,
    db: Session = Depends(database.get_db)
):
    user_info = get_current_user_from_token(request)
    username = user_info["username"]
    roles = user_info["roles"]

    if "admin" in roles:
        # Admins see all searches
        return db.query(models.SavedSearch).all()
    else:
        # Viewers and developers see only their own searches
        return db.query(models.SavedSearch).filter(models.SavedSearch.username == username).all()

@app.put("/searches/{search_id}", response_model=SavedSearchResponse)
async def update_saved_search(
    search_id: int,
    search_update: SavedSearchUpdate,
    request: Request,
    db: Session = Depends(database.get_db)
):
    user_info = get_current_user_from_token(request)
    username = user_info["username"]
    roles = user_info["roles"]

    db_search = db.query(models.SavedSearch).filter(models.SavedSearch.id == search_id).first()
    if not db_search:
        raise HTTPException(status_code=404, detail="Saved search not found")

    # Access control: Owner or Admin
    if db_search.username != username and "admin" not in roles:
        raise HTTPException(status_code=403, detail="Not authorized to update this search")

    if search_update.name is not None:
        db_search.name = search_update.name
    if search_update.query is not None:
        db_search.query = search_update.query

    db.commit()
    db.refresh(db_search)
    return db_search

@app.delete("/searches/{search_id}")
async def delete_saved_search(
    search_id: int,
    request: Request,
    db: Session = Depends(database.get_db)
):
    user_info = get_current_user_from_token(request)
    username = user_info["username"]
    roles = user_info["roles"]

    db_search = db.query(models.SavedSearch).filter(models.SavedSearch.id == search_id).first()
    if not db_search:
        raise HTTPException(status_code=404, detail="Saved search not found")

    # Access control: Owner or Admin
    if db_search.username != username and "admin" not in roles:
        raise HTTPException(status_code=403, detail="Not authorized to delete this search")

    db.delete(db_search)
    db.commit()
    return {"detail": "Saved search deleted successfully"}