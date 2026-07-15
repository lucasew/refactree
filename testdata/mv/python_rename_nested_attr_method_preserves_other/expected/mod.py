class Inner:
    def assist(self):
        return 1


class Other:
    def helper(self):
        return 2


class Box:
    def __init__(self):
        self.inner = Inner()
        self.other = Other()

    def use(self):
        return self.inner.helper() + self.other.helper()
