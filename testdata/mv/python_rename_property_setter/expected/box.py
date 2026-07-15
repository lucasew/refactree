class Box:
    def __init__(self):
        self._helper = 1
        self._stay = 2

    @property
    def assist(self):
        return self._helper

    @assist.setter
    def assist(self, v):
        self._helper = v

    @property
    def stay(self):
        return self._stay


def use(b: Box) -> int:
    b.assist = 3
    return b.assist + b.stay
