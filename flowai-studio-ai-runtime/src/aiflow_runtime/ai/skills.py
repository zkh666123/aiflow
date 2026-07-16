from __future__ import annotations
import ast,json,operator,secrets
import grpc
from aiflow.v1 import common_pb2,sandbox_pb2,sandbox_pb2_grpc
from aiflow_runtime.workflow.executors import NodeResult

OPS={ast.Add:operator.add,ast.Sub:operator.sub,ast.Mult:operator.mul,ast.Div:operator.truediv,ast.FloorDiv:operator.floordiv,ast.Mod:operator.mod,ast.Pow:operator.pow,ast.USub:operator.neg,ast.UAdd:operator.pos}
def calculate(expression:str)->int|float:
    def visit(node:ast.AST):
        if isinstance(node,ast.Expression):return visit(node.body)
        if isinstance(node,ast.Constant) and isinstance(node.value,(int,float)):return node.value
        if isinstance(node,ast.BinOp) and type(node.op) in OPS:return OPS[type(node.op)](visit(node.left),visit(node.right))
        if isinstance(node,ast.UnaryOp) and type(node.op) in OPS:return OPS[type(node.op)](visit(node.operand))
        raise ValueError("Expression contains unsupported syntax")
    return visit(ast.parse(expression,mode="eval"))

class SkillService:
    def __init__(self,address:str,token:str|None)->None:self.address=address;self.token=token
    async def execute_python(self,code:str)->dict[str,object]:
        async with grpc.aio.insecure_channel(self.address) as channel:
            stub=sandbox_pb2_grpc.SandboxServiceStub(channel);metadata=(("authorization",f"Bearer {self.token}"),) if self.token else None
            response=await stub.ExecutePython(sandbox_pb2.ExecutePythonRequest(context=common_pb2.RequestContext(request_id=secrets.token_hex(16),caller="python-backend"),code=code,limits=sandbox_pb2.SandboxLimits(timeout_millis=10000,memory_bytes=134217728,output_bytes=65536,fuel=100000000)),metadata=metadata)
            return {"status":response.status,"stdout":response.stdout,"stderr":response.stderr,"exitCode":response.exit_code,"durationMs":response.duration_millis}
    async def execute_node(self,node:dict,context:dict,inputs:dict)->NodeResult:
        data=node.get("data") or {};params=data.get("parameters") or {}
        if data.get("skillId")=="calculator":return NodeResult({"result":calculate(str(params.get("expression") or inputs.get("expression") or "0"))})
        code=str(params.get("code") or data.get("code") or "");return NodeResult(await self.execute_python(code))
