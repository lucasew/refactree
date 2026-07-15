class Helper:
    pass


class Stay:
    pass


class Box[T: (Helper, Stay)]:
    def __init__(self, v: T):
        self.v = v


def use(b: Box[Helper]) -> Helper:
    return b.v
