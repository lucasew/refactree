class Box:
    @property
    def helper(self):
        return 1

    @helper.setter
    def helper(self, v):
        pass

    def stay(self):
        return 2


def use(b: Box):
    return b.helper + b.stay()
