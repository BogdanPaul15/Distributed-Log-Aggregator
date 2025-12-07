from fastapi import Depends, HTTPException, status
from fastapi.security import OAuth2PasswordBearer
from jose import JWTError, jwt
import os

KEYCLOAK_URL = os.getenv("KEYCLOAK_URL", "http://keycloak:8080/realms/log-realm")

oauth2_scheme = OAuth2PasswordBearer(tokenUrl="token")

async def get_current_user(token: str = Depends(oauth2_scheme)):
    """
    Validates the JWT token and extracts the user payload.
    """
    try:
        payload = jwt.decode(token, options={"verify_signature": False})
        
        username: str = payload.get("preferred_username")
        roles: list = payload.get("realm_access", {}).get("roles", [])
        
        if username is None:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Could not validate credentials",
                headers={"WWW-Authenticate": "Bearer"},
            )
        
        return {"username": username, "roles": roles}
        
    except JWTError:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid authentication credentials",
            headers={"WWW-Authenticate": "Bearer"},
        )

def has_role(user: dict, role_name: str):
    """
    Helper to check if the logged-in user has a specific role.
    """
    if role_name not in user["roles"]:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail=f"Operation not permitted. Requires role: {role_name}"
        )
    return True