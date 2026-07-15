class Box:
    def _helper(self):
        return 1

    def _stay(self):
        return 2

    helper = property(_helper)
    stay = property(_stay)


def use(b: Box) -> int:
    return b.helper + b.stay
