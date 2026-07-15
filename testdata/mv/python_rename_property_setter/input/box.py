class Box:
    def __init__(self):
        self._helper = 1
        self._stay = 2

    @property
    def helper(self):
        return self._helper

    @helper.setter
    def helper(self, v):
        self._helper = v

    @property
    def stay(self):
        return self._stay


def use(b: Box) -> int:
    b.helper = 3
    return b.helper + b.stay
