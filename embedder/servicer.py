import grpc
from sentence_transformers import SentenceTransformer

from proto import embedder_pb2, embedder_pb2_grpc


class EmbedderServicer(embedder_pb2_grpc.EmbedderServicer):
    def __init__(self, model_name: str) -> None:
        self._model = SentenceTransformer(model_name)

    def Embed(self, request: embedder_pb2.EmbedRequest, context: grpc.ServicerContext) -> embedder_pb2.EmbedResponse:
        if not request.text:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "text is empty")

        vector = self._model.encode(request.text).tolist()
        return embedder_pb2.EmbedResponse(vector=vector)

    def EmbedBatch(self, request: embedder_pb2.EmbedBatchRequest, context: grpc.ServicerContext) -> embedder_pb2.EmbedBatchResponse:
        if not request.texts:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "texts is empty")

        vectors = self._model.encode(list(request.texts))
        return embedder_pb2.EmbedBatchResponse(
            vectors=[embedder_pb2.Vector(values=v.tolist()) for v in vectors]
        )
