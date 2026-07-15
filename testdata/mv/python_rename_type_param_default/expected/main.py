class Assist:
    pass


class Box[T = Assist]:
    def __init__(self, v: T | None = None):
        self.v = v


def use(b: Box) -> Assist | None:
    return b.v
