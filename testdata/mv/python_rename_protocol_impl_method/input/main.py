from typing import Protocol

class Worker(Protocol):
    def helper(self) -> int: ...
    def stay(self) -> int: ...

class Box(Worker):
    def helper(self) -> int:
        return 1
    def stay(self) -> int:
        return 2

def use(w: Worker) -> int:
    return w.helper() + w.stay()
