import pytest
import zmq
import zmq.asyncio
import json
import asyncio
from src.router import ZeroRouter

class MockEngine:
    async def generate(self, request_id: str, prompt: str, image_bytes_list: list[bytes]) -> dict:
        if prompt == "trigger_oom":
            raise Exception("Metal OOM")
        # Acknowledge image presence
        text_resp = f"Mock Response to {prompt}"
        if len(image_bytes_list) > 0:
            text_resp += f" with {len(image_bytes_list)} images"
        return {"request_id": request_id, "text": text_resp}

@pytest.fixture
def mock_engine():
    return MockEngine()

@pytest.mark.asyncio
async def test_router_success(mock_engine):
    import uuid
    socket_path = f"ipc:///tmp/test_{uuid.uuid4().hex[:8]}.sock"
    router = ZeroRouter(socket_path, mock_engine, hwm=100)
    
    task = asyncio.create_task(router.start())
    await asyncio.sleep(0.1) # Wait for bind
    
    ctx = zmq.asyncio.Context()
    client = ctx.socket(zmq.DEALER)
    client.connect(socket_path)
    
    metadata = {
        "request_id": "req-123",
        "prompt": "describe this",
        "trace_context": {}
    }
    
    await client.send_multipart([
        json.dumps(metadata).encode('utf-8'),
        b"fake_image_bytes", b"fake_image2"
    ])
    
    reply = await client.recv_multipart()
    assert len(reply) == 1
    resp = json.loads(reply[0].decode('utf-8'))
    
    assert resp["request_id"] == "req-123"
    assert resp["text"] == "Mock Response to describe this with 2 images"
    
    task.cancel()
    router.close()
    client.close()

@pytest.mark.asyncio
async def test_router_exception(mock_engine):
    import uuid
    socket_path = f"ipc:///tmp/test_{uuid.uuid4().hex[:8]}.sock"
    router = ZeroRouter(socket_path, mock_engine, hwm=100)
    
    task = asyncio.create_task(router.start())
    await asyncio.sleep(0.1)
    
    ctx = zmq.asyncio.Context()
    client = ctx.socket(zmq.DEALER)
    client.connect(socket_path)
    
    metadata = {
        "request_id": "req-999",
        "prompt": "trigger_oom"
    }
    
    await client.send_multipart([
        json.dumps(metadata).encode('utf-8')
    ])
    
    reply = await client.recv_multipart()
    resp = json.loads(reply[0].decode('utf-8'))
    
    assert resp["request_id"] == "req-999"
    assert "error" in resp
    assert "Metal OOM" in resp["error"]
    
    task.cancel()
    router.close()
    client.close()
