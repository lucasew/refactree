class Box:
    def helper(self):
        return 1

    def stay(self):
        return 2

    alias = helper
    p = property(helper)


def use(b: Box) -> int:
    return b.alias() + b.p + b.stay()
