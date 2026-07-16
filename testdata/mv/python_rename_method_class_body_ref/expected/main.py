class Box:
    def assist(self):
        return 1

    def stay(self):
        return 2

    alias = assist
    p = property(assist)


def use(b: Box) -> int:
    return b.alias() + b.p + b.stay()
