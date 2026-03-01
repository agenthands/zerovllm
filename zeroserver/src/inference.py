import io
import logging
import asyncio
import base64
from PIL import Image

try:
    from vllm_mlx.engine import BatchedEngine
except ImportError:
    BatchedEngine = None

logger = logging.getLogger(__name__)

class VisionEngine:
    def __init__(self, model_name: str, max_model_len: int = 4096):
        if BatchedEngine is None:
            logger.warning("vllm_mlx is not installed or failed to import. Inference will fail unless mocked.")
            self.engine = None
        else:
            try:
                from vllm_mlx.mllm_scheduler import MLLMSchedulerConfig
                original_init = MLLMSchedulerConfig.__init__
                def new_init(self, *args, **kwargs):
                    if 'prefill_step_size' not in kwargs:
                        kwargs['prefill_step_size'] = 8192
                    original_init(self, *args, **kwargs)
                MLLMSchedulerConfig.__init__ = new_init

                # Monkey-patch process_image_input to bypass Base64 encoding for PIL Images
                import vllm_mlx.models.mllm
                original_process_image_input = vllm_mlx.models.mllm.process_image_input
                def patched_process_image_input(image):
                    if isinstance(image, Image.Image):
                        return image
                    return original_process_image_input(image)
                vllm_mlx.models.mllm.process_image_input = patched_process_image_input
                
                import vllm_mlx.multimodal_processor
                if hasattr(vllm_mlx.multimodal_processor, 'process_image_input'):
                    vllm_mlx.multimodal_processor.process_image_input = patched_process_image_input
            except ImportError as e:
                logger.warning(f"Failed to patch vllm_mlx: {e}")

            logger.info(f"Loading vllm_mlx BatchedEngine model: {model_name}")
            self.engine = BatchedEngine(model_name, force_mllm=True)
            self.max_model_len = max_model_len
            
            # Start the engine asynchronously inside the event loop using create_task
            self._start_task = None

    async def _ensure_started(self):
        if self._start_task is None:
            self._start_task = asyncio.create_task(self.engine.start())
        await self._start_task

    async def generate(self, request_id: str, prompt: str, image_bytes_list: list[bytes]) -> dict:
        try:
            if self.engine is None:
                raise RuntimeError("BatchedEngine not initialized.")
            
            await self._ensure_started()

            images = []
            for b_img in image_bytes_list:
                img = Image.open(io.BytesIO(b_img)).convert("RGB")
                images.append(img)
                
            if images:
                content = []
                for img in images:
                    content.append({"type": "image", "image": img})
                content.append({"type": "text", "text": prompt})
                messages = [{"role": "user", "content": content}]
            else:
                messages = [{"role": "user", "content": prompt}]
            
            generator = self.engine.stream_chat(
                messages=messages,
                max_tokens=1024,
                temperature=0.2,
                images=images if images else None
            )

            text_out = []
            async for response in generator:
                if hasattr(response, "new_text") and response.new_text:
                    text_out.append(response.new_text)

            return {"request_id": request_id, "text": "".join(text_out)}

        except Exception as e:
            logger.error(f"Inference failed for {request_id}: {e}")
            return {"request_id": request_id, "error": f"Inference failed: {str(e)}"}


