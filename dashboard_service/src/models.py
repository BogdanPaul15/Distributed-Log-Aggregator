from sqlalchemy import Column, Integer, String, DateTime
from database import Base
import datetime

class UserProfile(Base):
    __tablename__ = "users"

    id = Column(Integer, primary_key=True, index=True)
    username = Column(String, unique=True, index=True)
    email = Column(String)
    role = Column(String, default="viewer")
    last_login = Column(DateTime, default=datetime.datetime.utcnow)