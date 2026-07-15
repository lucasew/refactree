class Box:
    __slots__ = ("assist", "stay")
    assist: int
    stay: int

    def __init__(self):
        self.assist = 1
        self.stay = 2

    def use(self):
        return self.assist + self.stay
