import requests
import time
import random
import os
import datetime

INGESTOR_URL = os.getenv("INGESTOR_URL", "http://log-ingestor:8000/ingest")
SERVICES = ["payment-service", "auth-service", "inventory-service", "checkout-service", "notification-service"]
LEVELS = ["INFO", "INFO", "INFO", "DEBUG", "DEBUG", "WARN", "ERROR", "FATAL"]
MESSAGES = {
    "payment-service": [
        "Transaction initiated", "Validating currency", "Payment gateway timeout", "Payment processed successfully", "Insufficient funds"
    ],
    "auth-service": [
        "User logged in", "User logged out", "Invalid password attempt", "Token refreshed", "Account locked"
    ],
    "inventory-service": [
        "Stock checked", "Item reserved", "Stock low warning", "Inventory sync complete", "Database connection lost"
    ],
    "checkout-service": [
        "Cart updated", "Checkout started", "Shipping address validated", "Order placed", "Payment confirmation failed"
    ],
    "notification-service": [
        "Email sent", "SMS queued", "Push notification failed", "Template rendered", "Rate limit exceeded"
    ]
}

def generate_log():
    service = random.choice(SERVICES)
    level = random.choice(LEVELS)
    message = random.choice(MESSAGES[service])
    
    if level == "ERROR" or level == "FATAL":
        message += f" (Error Code: {random.randint(1000, 9999)})"
    elif level == "INFO":
        message += f" [User ID: {random.randint(1, 500)}]"

    log_entry = {
        "service": service,
        "level": level,
        "message": message,
        "timestamp": datetime.datetime.utcnow().isoformat() + "Z"
    }
    
    return log_entry

def main():
    print(f"Starting log generator. Target: {INGESTOR_URL}")
    while True:
        try:
            log = generate_log()
            response = requests.post(INGESTOR_URL, json=log)
            if response.status_code == 200:
                print(f"Sent: {log['level']} - {log['service']}")
            else:
                print(f"Failed to send log: {response.status_code} - {response.text}")
        except Exception as e:
            print(f"Error sending log: {e}")
        
        time.sleep(random.uniform(0.5, 3.0))

if __name__ == "__main__":
    main()
