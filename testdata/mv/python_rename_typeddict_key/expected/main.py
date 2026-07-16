from typing import TypedDict

class Box(TypedDict):
    assist: int
    stay: int

def use(b: Box) -> int:
    return b["assist"] + b.get("assist", 0) + b["stay"]
