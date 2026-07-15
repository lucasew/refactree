class Helper:
    pass


class Box:
    def use[T: Helper](self, x: T) -> T:
        return x


def other(h: Helper) -> Helper:
    return h
