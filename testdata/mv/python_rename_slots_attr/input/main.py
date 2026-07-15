class Box:
    __slots__ = ("helper", "stay")
    helper: int
    stay: int

    def __init__(self):
        self.helper = 1
        self.stay = 2

    def use(self):
        return self.helper + self.stay
