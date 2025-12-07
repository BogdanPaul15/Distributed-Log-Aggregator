from fastapi import FastAPI, Depends, Request, Form, HTTPException
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates
from sqlalchemy.orm import Session
from jose import jwt
import models, database, os, requests, json
from urllib.parse import quote

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

# MOCK LOGS
MOCK_LOGS = [
    {"timestamp": "2025-11-28T10:00:01Z", "level": "INFO", "service": "payment-service", "message": "Transaction initiated for user 101"},
    {"timestamp": "2025-11-28T10:00:02Z", "level": "DEBUG", "service": "payment-service", "message": "Validating currency EUR"},
    {"timestamp": "2025-11-28T10:00:05Z", "level": "ERROR", "service": "checkout-service", "message": "Database connection timeout (5001ms)"},
    {"timestamp": "2025-11-28T10:02:10Z", "level": "WARN", "service": "inventory-service", "message": "Stock low for item #5521"},
    {"timestamp": "2025-11-28T10:05:00Z", "level": "INFO", "service": "auth-service", "message": "User testuser logged in successfully"},
    {"timestamp": "2025-11-28T10:06:23Z", "level": "FATAL", "service": "payment-gateway", "message": "Payment provider API unreachable"},
]

@app.get("/", response_class=HTMLResponse)
async def read_root(request: Request):
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
        
        app_role = "developer"
        if "viewer" in kc_roles or username == "viewer":
            app_role = "viewer"
        elif "admin" in kc_roles:
            app_role = "admin"

        user_db = db.query(models.UserProfile).filter(models.UserProfile.username == username).first()
        if not user_db:
            user_db = models.UserProfile(username=username, role=app_role)
            db.add(user_db)
        else:
            user_db.role = app_role
            
        db.commit()

        displayed_logs = MOCK_LOGS
        if user_db.role == 'viewer':
            displayed_logs = [l for l in displayed_logs if l['level'] in ['INFO', 'WARN']]

        response = templates.TemplateResponse("dashboard.html", {
            "request": request, 
            "user": username, 
            "role": user_db.role,
            "token": access_token,
            "decoded_token": pretty_payload,
            "logs": displayed_logs 
        })
        
        if id_token:
            response.set_cookie(key="id_token", value=id_token, httponly=True)
            
        return response
        
    except Exception as e:
        print(f"Login Error: {e}")
        return templates.TemplateResponse("dashboard.html", {
            "request": request, 
            "error": f"Login failed: {str(e)}"
        })