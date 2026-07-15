from abc import ABC, abstractmethod

class Worker(ABC):
    @abstractmethod
    def helper(self) -> int: ...
    @abstractmethod
    def stay(self) -> int: ...

class Box(Worker):
    def helper(self) -> int:
        return 1
    def stay(self) -> int:
        return 2

def use(w: Worker) -> int:
    return w.helper() + w.stay()

def use_box(b: Box) -> int:
    return b.helper() + b.stay()
