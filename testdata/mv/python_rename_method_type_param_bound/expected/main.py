class Assist:
    pass


class Box:
    def use[T: Assist](self, x: T) -> T:
        return x


def other(h: Assist) -> Assist:
    return h
