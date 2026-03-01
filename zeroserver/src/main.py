import argparse
import asyncio
import logging
import sys

from src.router import ZeroRouter
from src.inference import VisionEngine
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import ConsoleSpanExporter, SimpleSpanProcessor

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

def setup_telemetry():
    provider = TracerProvider()
    processor = SimpleSpanProcessor(ConsoleSpanExporter())
    provider.add_span_processor(processor)
    trace.set_tracer_provider(provider)

async def main_async(args):
    setup_telemetry()
    
    logger.info(f"Initializing AsyncLLMEngine for model: {args.model}")
    engine = VisionEngine(model_name=args.model, max_model_len=args.max_model_len)
    
    router = ZeroRouter(socket_path=args.socket, engine=engine, hwm=args.hwm)
    
    try:
        await router.start()
    except KeyboardInterrupt:
        logger.info("Shutting down...")
    except asyncio.CancelledError:
        pass
    finally:
        router.close()

def main():
    parser = argparse.ArgumentParser(description="ZeroMQ-vLLM Router")
    parser.add_argument("--model", type=str, required=True, help="HuggingFace model name (e.g., Qwen/Qwen1.5-1.8B)")
    parser.add_argument("--socket", type=str, required=True, help="ZMQ socket path (e.g., ipc:///tmp/zerovllm.sock)")
    parser.add_argument("--max-model-len", type=int, default=4096, help="Maximum model context length")
    parser.add_argument("--hwm", type=int, default=1000, help="ZMQ High Water Mark")
    
    args = parser.parse_args()
    
    asyncio.run(main_async(args))

if __name__ == "__main__":
    main()
