class Box:
    def __init__(self):
        self._v = 1

    @property
    def helper(self):
        return self._v

    @helper.setter
    def helper(self, v):
        self._v = v

    @helper.deleter
    def helper(self):
        self._v = 0

    def stay(self):
        return 2


def use(b: Box) -> int:
    b.helper = 3
    del b.helper
    return b.stay()
