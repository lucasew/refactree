from typing import TypedDict

class Box(TypedDict):
    helper: int
    stay: int

def use(b: Box) -> int:
    return b["helper"] + b.get("helper", 0) + b["stay"]
