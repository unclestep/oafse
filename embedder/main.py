import logging
import os
from concurrent import futures

import grpc
import torch

from proto import embedder_pb2_grpc
from servicer import EmbedderServicer

DEFAULT_ADDR = "[::]:50051"
DEFAULT_MODEL = "all-MiniLM-L6-v2"

def serve() -> None:
    addr = os.getenv("EMBEDDER_ADDR", DEFAULT_ADDR)
    model = os.getenv("EMBEDDER_MODEL", DEFAULT_MODEL)

    torch_threads = int(os.getenv("EMBEDDER_TORCH_THREADS", torch.get_num_threads()))
    torch.set_num_threads(torch_threads)

    grpc_workers = int(os.getenv("EMBEDDER_GRPC_WORKERS", 2))

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=grpc_workers))
    embedder_pb2_grpc.add_EmbedderServicer_to_server(EmbedderServicer(model), server)
    server.add_insecure_port(addr)
    server.start()

    logging.info(
        "EMBEDDER LISTENING ON %s model=%s torch_threads=%d grpc_workers=%d",
        addr, model, torch_threads, grpc_workers,
    )

    server.wait_for_termination()


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    serve()
