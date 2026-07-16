class Helper:
    pass


class Box[T = Helper]:
    def __init__(self, v: T | None = None):
        self.v = v


def use(b: Box) -> Helper | None:
    return b.v
