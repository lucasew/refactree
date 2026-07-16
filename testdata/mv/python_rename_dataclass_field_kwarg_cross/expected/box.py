from dataclasses import dataclass


@dataclass
class Box:
    assist: int
    stay: int = 0
