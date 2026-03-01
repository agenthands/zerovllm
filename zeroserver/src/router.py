import zmq
import zmq.asyncio
import asyncio
import orjson
import logging
from opentelemetry import trace
from .telemetry import extract_trace_context

logger = logging.getLogger(__name__)
tracer = trace.get_tracer(__name__)

class ZeroRouter:
    def __init__(self, socket_path: str, engine, hwm: int = 1000):
        self.socket_path = socket_path
        self.engine = engine
        self.ctx = zmq.asyncio.Context()
        self.socket = self.ctx.socket(zmq.ROUTER)
        
        if hwm > 0:
            self.socket.setsockopt(zmq.SNDHWM, hwm)
            self.socket.setsockopt(zmq.RCVHWM, hwm)
            
    async def start(self):
        self.socket.bind(self.socket_path)
        # Apply 0600 permissions to UDS socket
        if self.socket_path.startswith("ipc://"):
            import os
            import stat
            path = self.socket_path.replace("ipc://", "")
            try:
                os.chmod(path, stat.S_IRUSR | stat.S_IWUSR)
            except OSError as e:
                logger.warning(f"Failed to set socket permissions: {e}")

        logger.info(f"Router bound to {self.socket_path}")
        while True:
            try:
                frames = await self.socket.recv_multipart()
                asyncio.create_task(self.handle_request(frames))
            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.error(f"Error receiving zmq frames: {e}")

    async def handle_request(self, frames: list[bytes]):
        if len(frames) < 2:
            return

        identity = frames[0]
        meta_json = frames[1]
        image_frames = frames[2:]

        req_id = "unknown"
        try:
            metadata = orjson.loads(meta_json)
            req_id = metadata.get("request_id", "unknown")
            prompt = metadata.get("prompt", "")
            trace_ctx = metadata.get("trace_context", {})
            
            ctx = extract_trace_context(trace_ctx)
            
            with tracer.start_as_current_span("generate_vision", context=ctx) as span:
                span.set_attribute("request_id", req_id)
                span.set_attribute("image_count", len(image_frames))
                
                response = await self.engine.generate(req_id, prompt, image_frames)
                
                reply = [
                    identity,
                    orjson.dumps(response)
                ]
                await self.socket.send_multipart(reply)
                
        except orjson.JSONDecodeError:
            logger.error("Failed to decode metadata JSON")
            await self.send_error(identity, "unknown", "invalid metadata JSON")
        except Exception as e:
            logger.error(f"Error handling request: {e}")
            await self.send_error(identity, req_id, str(e))

    async def send_error(self, identity: bytes, req_id: str, err_msg: str):
        try:
            reply = [
                identity,
                orjson.dumps({"request_id": req_id, "error": err_msg})
            ]
            await self.socket.send_multipart(reply)
        except Exception as e:
            logger.error(f"Failed to send error reply: {e}")

    def close(self):
        self.socket.close()
        self.ctx.term()
