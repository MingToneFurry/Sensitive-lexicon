#!/usr/bin/env python3
import argparse
import base64
import binascii
import io
import json
import sys


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True)
    parser.add_argument("--gpu", action="store_true")
    parser.add_argument("--gpu-id", default="0")
    args = parser.parse_args()

    try:
        raw = sys.stdin.read()
        payload = json.loads(raw)
        image_b64 = payload.get("image_base64", "")
        if not image_b64:
            print(json.dumps({"error": "missing image_base64"}, ensure_ascii=False))
            return 0

        sys.path.insert(0, args.repo)

        import config as ocr_config  # type: ignore

        ocr_config.GPU_ID = args.gpu_id if args.gpu else "cpu"

        import model  # type: ignore
        import numpy as np
        from PIL import Image

        img_bytes = base64.b64decode(image_b64)
        img = Image.open(io.BytesIO(img_bytes)).convert("RGB")
        result = model.text_predict(np.array(img))
        text = " ".join([item.get("text", "") for item in result if item.get("text")])
        print(json.dumps({"text": text}, ensure_ascii=False))
        return 0
    except (json.JSONDecodeError, binascii.Error, ValueError, OSError, ImportError, RuntimeError) as exc:
        print(json.dumps({"error": str(exc)}, ensure_ascii=False))
        return 0


if __name__ == "__main__":
    raise SystemExit(main())
