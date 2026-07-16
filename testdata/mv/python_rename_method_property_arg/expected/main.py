class Box:
    def _assist(self):
        return 1

    def _stay(self):
        return 2

    helper = property(_assist)
    stay = property(_stay)


def use(b: Box) -> int:
    return b.helper + b.stay
