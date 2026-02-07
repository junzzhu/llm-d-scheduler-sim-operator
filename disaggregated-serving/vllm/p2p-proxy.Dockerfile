FROM python:3.11-slim

WORKDIR /app

# Install dependencies
RUN pip install --no-cache-dir quart aiohttp

# Copy proxy script
COPY p2p-proxy.py /app/proxy.py

# Make it executable
RUN chmod +x /app/proxy.py

# Expose proxy port
EXPOSE 8080

# Run proxy
CMD ["python", "/app/proxy.py"]