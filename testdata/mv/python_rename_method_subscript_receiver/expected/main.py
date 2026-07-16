class Box:
    def assist(self):
        return 1

    def stay(self):
        return 2


def use(xs):
    return xs[0].assist() + xs[0].stay()
