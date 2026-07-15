from typing import Protocol

class Worker(Protocol):
    def assist(self) -> int: ...
    def stay(self) -> int: ...

class Box(Worker):
    def assist(self) -> int:
        return 1
    def stay(self) -> int:
        return 2

def use(w: Worker) -> int:
    return w.assist() + w.stay()
