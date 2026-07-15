from dataclasses import dataclass, replace


@dataclass
class Box:
    assist: int
    stay: int = 0


def use(b: Box) -> Box:
    return replace(b, assist=3, stay=b.stay)
