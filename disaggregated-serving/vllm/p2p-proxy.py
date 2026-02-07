#!/usr/bin/env python3
"""
Simplified P2P NCCL Proxy Server for vLLM Disaggregated Serving
Routes requests between prefill and decode instances with proper request IDs
"""

import os
import uuid
import asyncio
import aiohttp
from quart import Quart, request, make_response

app = Quart(__name__)

# Configuration - will be set via environment variables
PREFILL_URL = os.getenv("PREFILL_URL", "http://vllm-prefill-same:8000")
DECODE_URL = os.getenv("DECODE_URL", "http://vllm-decode-same:8000")
PREFILL_ZMQ = os.getenv("PREFILL_ZMQ", "vllm-prefill-same:14579")
DECODE_ZMQ = os.getenv("DECODE_ZMQ", "vllm-decode-same:14579")

AIOHTTP_TIMEOUT = aiohttp.ClientTimeout(total=6 * 60 * 60)

def random_uuid() -> str:
    return str(uuid.uuid4().hex)

async def forward_request(url, data, request_id):
    """Forward request to vLLM instance with custom request ID"""
    async with aiohttp.ClientSession(timeout=AIOHTTP_TIMEOUT) as session:
        headers = {
            "X-Request-Id": request_id,
            "Content-Type": "application/json"
        }
        async with session.post(url=url, json=data, headers=headers) as response:
            if response.status == 200:
                async for chunk_bytes in response.content.iter_chunked(1024):
                    yield chunk_bytes
            else:
                error_text = await response.text()
                raise Exception(f"Request failed: {response.status} - {error_text}")

@app.route("/v1/completions", methods=["POST"])
@app.route("/v1/chat/completions", methods=["POST"])
async def handle_request():
    """Handle disaggregated serving request"""
    try:
        original_request_data = await request.get_json()
        
        # Create prefill request (max_tokens=1 for prefill only)
        prefill_request = original_request_data.copy()
        prefill_request["max_tokens"] = 1
        if "max_completion_tokens" in prefill_request:
            prefill_request["max_completion_tokens"] = 1
        
        # Generate special request ID for P2P NCCL transfer
        request_id = (
            f"___prefill_addr_{PREFILL_ZMQ}___decode_addr_"
            f"{DECODE_ZMQ}_{random_uuid()}"
        )
        
        print(f"Routing request {request_id}")
        print(f"  Prefill: {PREFILL_URL} (ZMQ: {PREFILL_ZMQ})")
        print(f"  Decode:  {DECODE_URL} (ZMQ: {DECODE_ZMQ})")
        
        # Step 1: Send to prefill (saves KV cache)
        async for _ in forward_request(
            f"{PREFILL_URL}{request.path}", 
            prefill_request, 
            request_id
        ):
            continue  # Consume prefill response
        
        print(f"Prefill complete for {request_id}, forwarding to decode...")
        
        # Step 2: Send to decode (loads KV cache and continues)
        generator = forward_request(
            f"{DECODE_URL}{request.path}", 
            original_request_data, 
            request_id
        )
        response = await make_response(generator)
        response.timeout = None
        
        return response
        
    except Exception as e:
        import traceback
        print(f"Error in proxy: {e}")
        traceback.print_exc()
        return {"error": str(e)}, 500

@app.route("/health", methods=["GET"])
async def health():
    """Health check endpoint"""
    return {"status": "healthy", "prefill": PREFILL_URL, "decode": DECODE_URL}

if __name__ == "__main__":
    print("Starting P2P NCCL Proxy Server")
    print(f"Prefill URL: {PREFILL_URL} (ZMQ: {PREFILL_ZMQ})")
    print(f"Decode URL:  {DECODE_URL} (ZMQ: {DECODE_ZMQ})")
    print("Listening on 0.0.0.0:8080")
    app.run(host="0.0.0.0", port=8080)