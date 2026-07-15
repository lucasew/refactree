class Inner:
    def helper(self):
        return 1

    def stay(self):
        return 2


class Box:
    def __init__(self):
        self.inner = Inner()

    def use(self):
        return self.inner.helper() + self.inner.stay()


def use(b: Box):
    return b.inner.helper() + b.inner.stay()
