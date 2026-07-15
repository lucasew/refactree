from abc import ABC, abstractmethod

class Worker(ABC):
    @abstractmethod
    def assist(self) -> int: ...
    @abstractmethod
    def stay(self) -> int: ...

class Box(Worker):
    def assist(self) -> int:
        return 1
    def stay(self) -> int:
        return 2

def use(w: Worker) -> int:
    return w.assist() + w.stay()

def use_box(b: Box) -> int:
    return b.assist() + b.stay()
