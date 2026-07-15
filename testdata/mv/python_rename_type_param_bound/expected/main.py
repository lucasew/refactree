class Assist:
    pass


class Stay:
    pass


class Box[T: Assist]:
    def __init__(self, v: T):
        self.v = v


def use(b: Box[Assist]) -> Assist:
    return b.v
