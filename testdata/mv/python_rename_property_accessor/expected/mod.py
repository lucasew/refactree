class Box:
    def __init__(self):
        self._v = 1

    @property
    def assist(self):
        return self._v

    @assist.setter
    def assist(self, v):
        self._v = v

    @assist.deleter
    def assist(self):
        self._v = 0

    def stay(self):
        return 2


def use(b: Box) -> int:
    b.assist = 3
    del b.assist
    return b.stay()
