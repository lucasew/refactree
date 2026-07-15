class Box:
    AMOUNT = 1
    COUNT = 2

    def use(self):
        return self.AMOUNT + Box.AMOUNT
